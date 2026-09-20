package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateID_Allowed(t *testing.T) {
	ids := []string{
		"550e8400-e29b-41d4-a716-446655440000", // UUID
		"__root__",                             // 予約コレクション ID
		"item1",
		"a",
		"A-B_c.d",
		"console", // 予約名への前方一致は正規のデバイス名ではない
		"con1",
		"com0",
		"com10",
		"lpt0",
		strings.Repeat("a", 128), // 上限ちょうど
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			if err := ValidateID(id); err != nil {
				t.Errorf("ValidateID(%q) = %v, want nil", id, err)
			}
		})
	}
}

func TestValidateID_Rejected(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"too long", strings.Repeat("a", 129)},
		{"parent traversal", "../x"},
		{"windows traversal", `..\x`},
		{"absolute path", "/abs"},
		{"drive path", `C:\x`},
		{"dot dot", ".."},
		{"dot", "."},
		{"leading dot", ".hidden"},
		{"trailing dot", "trailing."},
		{"slash inside", "a/b"},
		{"nul byte", "a\x00b"},
		{"space", "a b"},
		{"non ascii", "日本語"},
		{"reserved CON", "CON"},
		{"reserved con lower", "con"},
		{"reserved with ext", "CON.txt"},
		{"reserved com1 with ext", "com1.backup"},
		{"reserved lpt1 multi ext", "LPT1.x.y"},
		{"reserved nul", "nul"},
		{"reserved aux with ext", "AUX.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateID(tc.id)
			if err == nil {
				t.Fatalf("ValidateID(%q) = nil, want error", tc.id)
			}
			ve, ok := errors.AsType[*ValidationError](err)
			if !ok {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
			if ve.Field != "id" {
				t.Errorf("Field = %q, want id", ve.Field)
			}
		})
	}
}
