package testutil

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// ReadGolden は testdata の golden ファイルを読む。
func ReadGolden(tb testing.TB, path string) []byte {
	tb.Helper()
	// #nosec G304 -- path はテストが指定する testdata のファイル。
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read golden %s: %v", path, err)
	}
	return data
}

// AssertJSONEqual は got と want が同じ JSON 値であることを確かめる。
// キーの順序と空白は無視する。
func AssertJSONEqual(tb testing.TB, got, want []byte) {
	tb.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		tb.Fatalf("got is not valid JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		tb.Fatalf("want is not valid JSON: %v\n%s", err, want)
	}
	if !reflect.DeepEqual(g, w) {
		tb.Fatalf("JSON mismatch:\n got %s\nwant %s", got, want)
	}
}
