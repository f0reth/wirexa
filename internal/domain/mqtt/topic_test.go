package mqttdomain

import (
	"errors"
	"strings"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

func TestValidateTopicName(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		wantErr bool
	}{
		{name: "通常のトピック", topic: "sensors/temp"},
		{name: "先頭と末尾の区切り", topic: "/a/"},
		{name: "空の階層", topic: "a//b"},
		{name: "マルチバイト", topic: "センサー/温度"},
		{name: "空", topic: "", wantErr: true},
		{name: "+ を含む", topic: "a/+", wantErr: true},
		{name: "# を含む", topic: "a/#", wantErr: true},
		{name: "階層の一部に +", topic: "a+b", wantErr: true},
		{name: "NUL を含む", topic: "a\x00b", wantErr: true},
		{name: "長すぎる", topic: strings.Repeat("a", maxTopicBytes+1), wantErr: true},
		{name: "上限ちょうど", topic: strings.Repeat("a", maxTopicBytes)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertTopicErr(t, ValidateTopicName(tc.topic), tc.wantErr)
		})
	}
}

func TestValidateTopicFilter(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		wantErr bool
	}{
		{name: "通常のトピック", filter: "sensors/temp"},
		{name: "# だけ", filter: "#"},
		{name: "+ だけ", filter: "+"},
		{name: "末尾の #", filter: "sensors/#"},
		{name: "中間の +", filter: "sensors/+/temp"},
		{name: "+ と #", filter: "+/+/#"},
		{name: "空の階層と +", filter: "/+"},
		{name: "空", filter: "", wantErr: true},
		{name: "中間の #", filter: "a/#/b", wantErr: true},
		{name: "階層の一部に #", filter: "a/b#", wantErr: true},
		{name: "階層の一部に +", filter: "a+/b", wantErr: true},
		{name: "+ の後ろに文字", filter: "a/+b", wantErr: true},
		{name: "## ", filter: "a/##", wantErr: true},
		{name: "NUL を含む", filter: "a/\x00", wantErr: true},
		{name: "長すぎる", filter: strings.Repeat("a", maxTopicBytes+1), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertTopicErr(t, ValidateTopicFilter(tc.filter), tc.wantErr)
		})
	}
}

func assertTopicErr(t *testing.T, err error, wantErr bool) {
	t.Helper()
	if !wantErr {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	var ve *cmn.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	if ve.Field != "topic" {
		t.Errorf("Field = %q, want topic", ve.Field)
	}
}
