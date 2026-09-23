package httpinfra

import (
	"bytes"
	"errors"
	"io"
	"sync"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// bodySegment は送信ボディの 1 区間。file が nil ならメモリ上の data を、
// そうでなければ開いたファイルの先頭から size バイトを送る。
type bodySegment struct {
	file *sharedFile
	data []byte
	size int64
}

// bodySegments はメモリ上のバイト列とファイルの区間を並べた送信ボディ。
// ファイルの内容はメモリに載せず、リーダーが読むときにハンドルから読む。
// ファイル区間のハンドルの参照を 1 つずつ持ち、release で外す。
type bodySegments struct {
	segs     []bodySegment
	length   int64
	released bool
}

// newFileBody はファイル区間 1 つだけのボディを返す。file.File の参照はボディが引き継ぐ。
func newFileBody(file domain.OpenedSelectedFile) *bodySegments {
	b := &bodySegments{}
	b.appendFile(file)
	return b
}

// Write は末尾のバイト区間に p を追記する。multipart.Writer の出力先として使い、失敗しない。
func (b *bodySegments) Write(p []byte) (int, error) {
	if n := len(b.segs); n > 0 && b.segs[n-1].file == nil {
		b.segs[n-1].data = append(b.segs[n-1].data, p...)
	} else {
		b.segs = append(b.segs, bodySegment{data: bytes.Clone(p)})
	}
	b.length += int64(len(p))
	return len(p), nil
}

// appendFile はファイル区間を足す。長さは開いた時点のサイズで、file.File の参照はボディが引き継ぐ。
func (b *bodySegments) appendFile(file domain.OpenedSelectedFile) {
	b.segs = append(b.segs, bodySegment{file: newSharedFile(file.File), size: file.Size})
	b.length += file.Size
}

// release はボディ自身が持つハンドルの参照を外す。リーダーが残っていれば、
// ハンドルはそのリーダーがすべて閉じられたときに閉じる。2 回目以降の呼び出しは何もしない。
func (b *bodySegments) release() {
	if b.released {
		return
	}
	b.released = true
	for _, seg := range b.segs {
		if seg.file != nil {
			seg.file.release()
		}
	}
}

// newReader は先頭から読むリーダーを返す。リーダーは読むファイルの参照を持ち、Close で外す。
// ファイルの読み込みで起きたエラーは errs に記録する。
func (b *bodySegments) newReader(errs *bodyError) (io.ReadCloser, error) {
	var files []*sharedFile
	for _, seg := range b.segs {
		if seg.file == nil {
			continue
		}
		if !seg.file.acquire() {
			for _, f := range files {
				f.release()
			}
			return nil, domain.ErrSelectedFileUnavailable
		}
		files = append(files, seg.file)
	}
	return &segmentReader{segs: b.segs, files: files, errs: errs}, nil
}

// getBody は http.Request.GetBody を返す。307/308 やトランスポートの再試行で送り直すときも
// 開き直さずに同じハンドルを先頭から読み直す。読み直す前に変更を確かめ、変わっていれば送り直さない。
func (b *bodySegments) getBody(errs *bodyError) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		for _, seg := range b.segs {
			if seg.file == nil {
				continue
			}
			if err := seg.file.h.CheckUnchanged(); err != nil {
				err = classifyFileError(err)
				errs.record(err)
				return nil, err
			}
		}
		rc, err := b.newReader(errs)
		if err != nil {
			errs.record(err)
			return nil, err
		}
		return rc, nil
	}
}

// sharedFile は 1 つのハンドルを、ボディとそれを読むすべてのリーダーで共有する。
// トランスポートは Do が戻った後に別の goroutine でボディを閉じることがあるため、
// 最後の参照が外れたときに 1 回だけ閉じる。
type sharedFile struct {
	h    domain.SelectedFileHandle
	mu   sync.Mutex
	refs int
}

func newSharedFile(h domain.SelectedFileHandle) *sharedFile {
	return &sharedFile{h: h, refs: 1}
}

// acquire は参照を 1 つ増やす。既に閉じていれば false を返す。
func (f *sharedFile) acquire() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refs == 0 {
		return false
	}
	f.refs++
	return true
}

// release は参照を 1 つ外し、最後の参照ならハンドルを閉じる。
func (f *sharedFile) release() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refs == 0 {
		return
	}
	f.refs--
	if f.refs == 0 {
		_ = f.h.Close() //nolint:errcheck // 読み取り専用ハンドルの後始末
	}
}

// segmentReader は区間を順に読む。ファイル区間は ReadAt で読むため、
// 同じハンドルを読む他のリーダーと読み位置が混ざらない。
type segmentReader struct {
	err       error
	errs      *bodyError
	segs      []bodySegment
	files     []*sharedFile
	idx       int
	off       int64
	closeOnce sync.Once
}

func (r *segmentReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	for r.idx < len(r.segs) {
		seg := r.segs[r.idx]
		if seg.file == nil {
			if r.off < int64(len(seg.data)) {
				n := copy(p, seg.data[r.off:])
				r.off += int64(n)
				return n, nil
			}
			r.next()
			continue
		}
		n, err := r.readFile(seg, p)
		if err != nil {
			r.err = classifyFileError(err)
			r.errs.record(r.err)
			return 0, r.err
		}
		r.off += int64(n)
		if r.off == seg.size {
			r.next()
		}
		if n > 0 {
			return n, nil
		}
	}
	return 0, io.EOF
}

// readFile はファイル区間の残りから p に読む。区間を読み切るときは、最後のバイトを返す前に
// CheckUnchanged で変更を確かめ、失敗すればそのバイトを返さずにエラーにする。これで変更に
// 気付いたときのボディは Content-Length に届かず、長さの合った誤った内容を送り切ることはない。
func (r *segmentReader) readFile(seg bodySegment, p []byte) (int, error) {
	if remaining := seg.size - r.off; int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n := 0
	if len(p) > 0 {
		var err error
		n, err = seg.file.h.ReadAt(p, r.off)
		if n < len(p) {
			// 基準のサイズに達する前の EOF は、検査するまでもなくファイルが縮んだとみなす。
			if err == nil || errors.Is(err, io.EOF) {
				return 0, domain.ErrSelectedFileChanged
			}
			return 0, err
		}
	}
	if r.off+int64(n) == seg.size {
		if err := seg.file.h.CheckUnchanged(); err != nil {
			return 0, err
		}
	}
	return n, nil
}

func (r *segmentReader) next() {
	r.idx++
	r.off = 0
}

// Close はリーダーが持つファイルの参照を外す。二重に呼んでも安全。
func (r *segmentReader) Close() error {
	r.closeOnce.Do(func() {
		for _, f := range r.files {
			f.release()
		}
	})
	return nil
}

// classifyFileError はファイルの読み込みエラーを、パスを含まない domain エラーのどちらかにする。
// ハンドルが既に置き換えているが、net/http へ OS のエラーが届かないよう、ここでも狭める。
func classifyFileError(err error) error {
	if errors.Is(err, domain.ErrSelectedFileChanged) {
		return domain.ErrSelectedFileChanged
	}
	return domain.ErrSelectedFileUnavailable
}

// bodyError はリクエスト単位で、ファイルの読み込みで最初に起きたエラーを覚えておく。
// net/http はボディの読み込みエラーを HTTP/1.1 と HTTP/2 で違う形に包むため、
// client.Do が失敗したときはこちらを返す。
type bodyError struct {
	err error
	mu  sync.Mutex
}

func (e *bodyError) record(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.err == nil {
		e.err = err
	}
}

func (e *bodyError) get() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.err
}
