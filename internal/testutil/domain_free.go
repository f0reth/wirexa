package testutil

import (
	"reflect"
	"strings"
	"testing"
)

// DomainPkg は domain 層のパッケージパスの接頭辞。AssertNoTypesFrom に渡して使う。
const DomainPkg = "github.com/f0reth/Wirexa/internal/domain"

// AssertNoTypesFrom は value の型をフィールド・スライスと配列の要素・map のキーと値・
// ポインタの指す先まで再帰的にたどり、pkgPaths (またはその配下) のパッケージの struct 型が
// どの階層にも出てこないことを確かめる。
//
// 永続化 DTO が domain 型を埋め込んでいないことの検査に使う。埋め込むとその部分の保存形式が
// domain の json タグで決まり、RPC の形式と保存形式を独立させられなくなる。
// 名前付きの基本型 (string の別名など) は json の形が基本型と同じなので検査の対象にしない。
func AssertNoTypesFrom(tb testing.TB, value any, pkgPaths ...string) {
	tb.Helper()
	w := &typeWalker{tb: tb, pkgPaths: pkgPaths, seen: map[reflect.Type]bool{}}
	typ := reflect.TypeOf(value)
	w.walk(typ, typ.String())
}

type typeWalker struct {
	tb       testing.TB
	seen     map[reflect.Type]bool
	pkgPaths []string
}

func (w *typeWalker) walk(typ reflect.Type, path string) {
	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		w.walk(typ.Elem(), path+"[]")
	case reflect.Map:
		w.walk(typ.Key(), path+"{key}")
		w.walk(typ.Elem(), path+"{value}")
	case reflect.Struct:
		if w.inPkgs(typ.PkgPath()) {
			w.tb.Errorf("%s has type %s from %s", path, typ, typ.PkgPath())
			return
		}
		if w.seen[typ] {
			return
		}
		w.seen[typ] = true
		for f := range typ.Fields() {
			w.walk(f.Type, path+"."+f.Name)
		}
	}
}

func (w *typeWalker) inPkgs(pkg string) bool {
	for _, p := range w.pkgPaths {
		if pkg == p || strings.HasPrefix(pkg, p+"/") {
			return true
		}
	}
	return false
}
