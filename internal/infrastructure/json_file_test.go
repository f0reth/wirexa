package infrastructure

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0reth/Wirexa/internal/domain"
)

func TestWriteJSONFile_IndentIsTwoSpaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	v := struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}{Name: "wirexa", N: 1}

	if err := WriteJSONFile(path, v, 0o600); err != nil {
		t.Fatalf("WriteJSONFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "{\n  \"name\": \"wirexa\",\n  \"n\": 1\n}"
	if string(got) != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestWriteJSONFile_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	want := []string{"a", "b", "c"}

	if err := WriteJSONFile(path, want, 0o600); err != nil {
		t.Fatalf("WriteJSONFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var got []string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestWriteJSONFile_MarshalErrorLeavesNoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")

	// chan はマーシャルできないため MarshalIndent が失敗する。
	if err := WriteJSONFile(path, make(chan int), 0o600); err == nil {
		t.Fatalf("expected marshal error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("marshal failure must not create a file")
	}
}

func TestReadJSONFile(t *testing.T) {
	type item struct {
		Name string `json:"name"`
	}
	dir := t.TempDir()

	t.Run("未作成なら found=false で nil を返す", func(t *testing.T) {
		v, found, err := ReadJSONFile[item](filepath.Join(dir, "absent.json"))
		if err != nil || found || v != (item{}) {
			t.Fatalf("ReadJSONFile = (%+v, %v, %v), want zero, false, nil", v, found, err)
		}
	})

	t.Run("正常に読める", func(t *testing.T) {
		path := filepath.Join(dir, "valid.json")
		if err := os.WriteFile(path, []byte(`{"name":"wirexa"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		v, found, err := ReadJSONFile[item](path)
		if err != nil || !found || v.Name != "wirexa" {
			t.Fatalf("ReadJSONFile = (%+v, %v, %v), want {wirexa}, true, nil", v, found, err)
		}
	})

	t.Run("破損なら ErrCorruptData", func(t *testing.T) {
		path := filepath.Join(dir, "corrupt.json")
		if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ReadJSONFile[item](path); !errors.Is(err, domain.ErrCorruptData) {
			t.Fatalf("ReadJSONFile on corrupt file: want ErrCorruptData, got %v", err)
		}
	})

	// 読み込み自体の失敗 (ここではパスがディレクトリ) は破損と区別する。
	t.Run("読み込み失敗は ErrCorruptData にならない", func(t *testing.T) {
		path := filepath.Join(dir, "unreadable.json")
		if err := os.Mkdir(path, 0o750); err != nil {
			t.Fatal(err)
		}
		_, _, err := ReadJSONFile[item](path)
		if err == nil {
			t.Fatal("ReadJSONFile on unreadable path should fail")
		}
		if errors.Is(err, domain.ErrCorruptData) {
			t.Fatalf("unreadable file must not be reported as corrupt: %v", err)
		}
	})
}
