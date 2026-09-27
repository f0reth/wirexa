package httpinfra

import (
	"strings"
	"testing"
)

func TestLongPath(t *testing.T) {
	long := `C:\` + strings.Repeat(`a\`, 130) + "f.txt"
	tests := []struct{ in, want string }{
		{in: `C:\short\f.txt`, want: `C:\short\f.txt`},
		{in: long, want: `\\?\` + long},
		{in: `\\server\share\` + long[3:], want: `\\?\UNC\server\share\` + long[3:]},
		{in: `\\?\` + long, want: `\\?\` + long},
		// 相対パスは長くても変換しない。
		{in: long[3:], want: long[3:]},
		{in: `C:` + long[3:], want: `C:` + long[3:]},
	}
	for _, tc := range tests {
		if got := longPath(tc.in); got != tc.want {
			t.Errorf("longPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsWindowsAbs(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{`C:\a`, true},
		{`c:/a`, true},
		{`\\server\share`, true},
		{`//server/share`, true},
		{`\\?\C:\a`, true},
		{`C:a`, false},
		{`\a`, false},
		{`a\b`, false},
		{`1:\a`, false},
		{``, false},
	}
	for _, tc := range tests {
		if got := isWindowsAbs(tc.in); got != tc.want {
			t.Errorf("isWindowsAbs(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
