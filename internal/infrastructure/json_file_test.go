package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
