package store

import (
	"errors"
	"path/filepath"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

// 設定データの分類と復旧方針。この表を唯一の定義とし、CLAUDE.md にも同じ内容を載せる。
//
//	| 分類 | 対象ファイル | 破損時 | 退避失敗時 | 破損以外の読み込み失敗 |
//	| --- | --- | --- | --- | --- |
//	| 必須 (ユーザーが作成し、再生成できない) | collections/*.json, mqtt-profiles/*.json, udp-targets/*.json (JSONStore) | そのファイルだけ退避してスキップし、残りで起動する | スキップし、元ファイルは残す。起動時に自動作成する __root__ は、ファイルが存在する限り作り直さない | ファイル単位ならスキップ。ディレクトリ自体を読めなければ起動失敗 |
//	| best effort (再生成できないが、失っても作業は続けられる) | openapi-recents.json | 退避して空の状態から始める | 空で始め、このセッションでは保存しない | 空で始め、このセッションでは保存しない |
//	| 再生成可能 (他のデータや既定値から作り直せる) | sidebar_layout.json (← collections), window-state.json (← 既定サイズ) | 退避して再生成する | 再生成した内容で上書きしてよい | 再生成した値で動作を続け、ファイルは退避しない |
//
// 共通ルール:
//   - 破損 (JSON として解釈できない = cmn.ErrCorruptData) と読み込み失敗 (I/O エラー) を区別し、
//     退避の対象は破損したファイルだけとする。
//   - 退避先は infrastructure.QuarantineFile (<path>.corrupt、既存なら <path>.corrupt.<unixnano>) に統一する。
//   - 「読み込めなかった」を「存在しない」と同一視しない。必須データを自動作成・初期化するときは、
//     ファイルが本当に無いことを確かめてから書く。
//
// 単一ファイル型の best effort / 再生成可能データは LoadSingleFile で読み込む。
// window-state.json は application 層を経由しないため、infrastructure 側で同じ方針を直接実装する。

// SingleFileRepository は一覧全体を 1 ファイルで読み書きする設定の読み込み・退避ポート。
// domain.SidebarLayoutRepository / openapidomain.RecentRepository が構造的に充足する。
type SingleFileRepository[T any] interface {
	Load() (T, error)
	cmn.Quarantiner
}

// RecoveryPolicy は破損したファイルを退避できなかったときの扱いを表す。
type RecoveryPolicy int

const (
	// PolicyBestEffort は退避できなければこのセッションでは保存しない。
	PolicyBestEffort RecoveryPolicy = iota
	// PolicyRegenerable は退避できなくても再生成した内容で上書きしてよい。
	PolicyRegenerable
)

// LoadSingleFile は repo.Load の結果を policy に従って解釈する。name はログに残すファイルの識別名。
//   - 成功: (値, persist=true, nil)
//   - 破損: 退避してログを残し (ゼロ値, persist, nil)。persist は退避成功なら true、
//     退避に失敗した場合は PolicyRegenerable なら true、PolicyBestEffort なら false
//   - 破損以外の読み込み失敗: 退避せず (ゼロ値, false, err)。続行するかは呼び出し側が決める
//
// logger は nil を許容し、その場合ログ出力をスキップする。
func LoadSingleFile[T any](repo SingleFileRepository[T], policy RecoveryPolicy, logger cmn.Logger, name string) (T, bool, error) {
	v, err := repo.Load()
	if err == nil {
		return v, true, nil
	}
	var zero T
	if !errors.Is(err, cmn.ErrCorruptData) {
		return zero, false, err
	}

	dest, qerr := repo.Quarantine()
	if qerr != nil {
		if policy == PolicyRegenerable {
			logError(logger, "failed to quarantine corrupt file, it will be overwritten with regenerated data",
				"file", name, "error", err, "quarantineError", qerr)
			return zero, true, nil
		}
		logError(logger, "failed to quarantine corrupt file, it will not be saved this session",
			"file", name, "error", err, "quarantineError", qerr)
		return zero, false, nil
	}
	logError(logger, "quarantined corrupt file, starting empty",
		"file", name, "quarantined", filepath.Base(dest), "error", err)
	return zero, true, nil
}

// logError は logger が設定されている場合だけエラーを記録する。
func logError(logger cmn.Logger, msg string, args ...any) {
	if logger != nil {
		logger.Error(msg, args...)
	}
}
