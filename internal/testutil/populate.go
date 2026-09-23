package testutil

import (
	"fmt"
	"reflect"
	"testing"
)

// populateMaxDepth は自己参照する型を埋める段数の上限。同じ struct 型は 2 段目までだけ埋め、
// それより深い段のスライス・map・ポインタは nil のままにする。
const populateMaxDepth = 2

// populateLen はスライスと map に入れる要素数。先頭だけコピーする書き漏れも見つけるため 2 つ入れる。
const populateLen = 2

// Populate は ptr が指す値の公開フィールドを、リフレクションですべてゼロ値以外の値で埋める。
//
// 永続化 DTO の往復テストで使う。検査対象のフィールドをテストのコードではなく型から決めるため、
// domain 型にフィールドを足せばテストを書き換えなくても次の実行から検査に入る。
//
//   - string (名前付きを含む): フィールドのパスから作る一意の文字列 (例: "Body.FormData[1].File.Name")
//   - 整数・浮動小数: 呼び出しごとに 1 から数える、フィールドごとに違う値
//   - bool: true
//   - struct: 公開フィールドを再帰的に埋める。非公開フィールドには触れない
//   - スライス・map: 要素を 2 つ入れる。ポインタ: 値を確保して指す先を埋める
//   - 自己参照する型: 同じ struct 型は 2 段目までだけ埋め、それより深い段は nil のままにする
//   - interface・func・chan など: 扱えないので Fatalf で止める
func Populate(tb testing.TB, ptr any) {
	tb.Helper()
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		tb.Fatalf("Populate: want a non-nil pointer, got %T", ptr)
		return
	}
	p := &populator{tb: tb, depth: map[reflect.Type]int{}}
	p.fill(v.Elem(), v.Elem().Type().Name())
}

type populator struct {
	tb      testing.TB
	depth   map[reflect.Type]int
	counter int
	failed  bool
}

func (p *populator) next() int {
	p.counter++
	return p.counter
}

func (p *populator) fill(v reflect.Value, path string) {
	if p.failed {
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(path)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		v.Set(reflect.ValueOf(p.next()).Convert(v.Type()))
	case reflect.Struct:
		p.depth[v.Type()]++
		defer func() { p.depth[v.Type()]-- }()
		for f, fv := range v.Fields() {
			if f.IsExported() {
				p.fill(fv, joinPath(path, f.Name))
			}
		}
	case reflect.Pointer:
		if p.tooDeep(v.Type().Elem()) {
			return
		}
		elem := reflect.New(v.Type().Elem())
		p.fill(elem.Elem(), path)
		v.Set(elem)
	case reflect.Slice:
		if p.tooDeep(v.Type().Elem()) {
			return
		}
		s := reflect.MakeSlice(v.Type(), populateLen, populateLen)
		for i := range populateLen {
			p.fill(s.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
		v.Set(s)
	case reflect.Array:
		for i := range v.Len() {
			p.fill(v.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.Map:
		if p.tooDeep(v.Type().Key()) || p.tooDeep(v.Type().Elem()) {
			return
		}
		m := reflect.MakeMapWithSize(v.Type(), populateLen)
		for i := range populateLen {
			key := reflect.New(v.Type().Key()).Elem()
			p.fill(key, fmt.Sprintf("%s{key%d}", path, i))
			val := reflect.New(v.Type().Elem()).Elem()
			p.fill(val, fmt.Sprintf("%s{value%d}", path, i))
			m.SetMapIndex(key, val)
		}
		v.Set(m)
	default:
		p.failed = true
		p.tb.Fatalf("Populate: unsupported kind %s at %s (%s)", v.Kind(), path, v.Type())
	}
}

// tooDeep は t (ポインタを外した先) が struct で、既に上限の段数まで埋めている途中なら true を返す。
func (p *populator) tooDeep(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct && p.depth[t] >= populateMaxDepth
}

func joinPath(base, name string) string {
	if base == "" {
		return name
	}
	return base + "." + name
}
