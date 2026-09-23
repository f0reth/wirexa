package testutil

import (
	"fmt"
	"reflect"
	"testing"
)

// recordingTB は Fatalf / Errorf を止めずに記録する testing.TB。失敗するべき入力の検査に使う。
type recordingTB struct {
	testing.TB
	fatals []string
	errors []string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, fmt.Sprintf(format, args...))
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

type namedString string

type populateInner struct {
	Label string
	Count uint16
}

type populateSample struct {
	Ptr     *populateInner
	Map     map[string]string
	IntMap  map[namedString]int
	Name    string
	Named   namedString
	Inner   populateInner
	Items   []populateInner
	Strings []string
	Ptrs    []*populateInner
	Array   [2]int32
	Int     int
	Int64   int64
	Float   float64
	Float32 float32
	Byte    byte
	Flag    bool
	hidden  string
}

// assertFilled は v の公開フィールドがすべてゼロ値以外で、スライスと map が 2 要素ずつ持つことを確かめる。
func assertFilled(t *testing.T, v reflect.Value, path string) {
	t.Helper()
	if v.IsZero() {
		t.Errorf("%s is zero", path)
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		for f, fv := range v.Fields() {
			if f.IsExported() {
				assertFilled(t, fv, path+"."+f.Name)
			}
		}
	case reflect.Pointer:
		assertFilled(t, v.Elem(), path)
	case reflect.Slice, reflect.Array:
		if v.Len() != 2 {
			t.Errorf("%s has %d elements, want 2", path, v.Len())
		}
		for i := range v.Len() {
			assertFilled(t, v.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.Map:
		if v.Len() != 2 {
			t.Errorf("%s has %d entries, want 2", path, v.Len())
		}
		for _, k := range v.MapKeys() {
			assertFilled(t, k, path+"{key}")
			assertFilled(t, v.MapIndex(k), path+"{value}")
		}
	}
}

// leaves は v に含まれる基本型の値をすべて集める。
func leaves(v reflect.Value, out *[]any) {
	switch v.Kind() {
	case reflect.Struct:
		for f, fv := range v.Fields() {
			if f.IsExported() {
				leaves(fv, out)
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			leaves(v.Elem(), out)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			leaves(v.Index(i), out)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			leaves(k, out)
			leaves(v.MapIndex(k), out)
		}
	case reflect.String:
		*out = append(*out, v.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		*out = append(*out, v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		*out = append(*out, int64(v.Uint()))
	case reflect.Float32, reflect.Float64:
		*out = append(*out, int64(v.Float()))
	}
}

func TestPopulate_FillsEveryExportedField(t *testing.T) {
	var v populateSample
	Populate(t, &v)

	assertFilled(t, reflect.ValueOf(v), "populateSample")
	if v.hidden != "" {
		t.Errorf("unexported field was touched: %q", v.hidden)
	}
}

// 値を取り違えるコピー (A に B の値を入れる) も往復テストで検出できるよう、フィールドごとに違う値が入る。
func TestPopulate_GivesDistinctValues(t *testing.T) {
	var v populateSample
	Populate(t, &v)

	var all []any
	leaves(reflect.ValueOf(v), &all)
	seen := map[any]bool{}
	for _, x := range all {
		if seen[x] {
			t.Errorf("value %v appears more than once", x)
		}
		seen[x] = true
	}
	if v.Items[1].Label != "populateSample.Items[1].Label" {
		t.Errorf("string is not derived from the field path: %q", v.Items[1].Label)
	}
}

type populateNode struct {
	Parent   *populateNode
	Name     string
	Children []*populateNode
}

// 自己参照する型は 2 段目までだけ埋め、それより深い段は nil のままにして止まる。
func TestPopulate_StopsOnSelfReference(t *testing.T) {
	var n populateNode
	Populate(t, &n)

	if len(n.Children) != 2 || n.Parent == nil {
		t.Fatalf("first level must be filled: %+v", n)
	}
	for i, c := range n.Children {
		if c.Name == "" {
			t.Errorf("Children[%d].Name is empty", i)
		}
		if c.Children != nil || c.Parent != nil {
			t.Errorf("Children[%d] must stop at the second level: %+v", i, c)
		}
	}
	if n.Parent.Parent != nil || n.Parent.Children != nil {
		t.Errorf("Parent must stop at the second level: %+v", n.Parent)
	}
}

func TestPopulate_FailsOnUnsupportedKinds(t *testing.T) {
	tests := map[string]any{
		"interface": &struct{ V any }{},
		"func":      &struct{ F func() }{},
		"chan":      &struct{ C chan int }{},
		"non-ptr":   struct{ S string }{},
	}
	for name, v := range tests {
		t.Run(name, func(t *testing.T) {
			rec := &recordingTB{TB: t}
			Populate(rec, v)
			if len(rec.fatals) == 0 {
				t.Fatal("Populate must fail")
			}
		})
	}
}
