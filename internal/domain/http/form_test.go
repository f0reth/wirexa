package httpdomain

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseFormPairs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []FormRow
	}{
		{
			name:    "空文字列は行なし",
			content: "",
			want:    nil,
		},
		{
			name:    "行順を保つ（url.Values と違いソートしない）",
			content: "z=1&a=2&m=3",
			want: []FormRow{
				{Key: "z", Value: "1", Enabled: true},
				{Key: "a", Value: "2", Enabled: true},
				{Key: "m", Value: "3", Enabled: true},
			},
		},
		{
			name:    "重複キーを潰さない",
			content: "k=1&k=2",
			want: []FormRow{
				{Key: "k", Value: "1", Enabled: true},
				{Key: "k", Value: "2", Enabled: true},
			},
		},
		{
			name:    "値に = を含む",
			content: "k=a=b",
			want:    []FormRow{{Key: "k", Value: "a=b", Enabled: true}},
		},
		{
			name:    "= が無い要素は値を空にする",
			content: "k",
			want:    []FormRow{{Key: "k", Value: "", Enabled: true}},
		},
		{
			name:    "パーセントエンコードと + をデコードする",
			content: "a%20b=c+d",
			want:    []FormRow{{Key: "a b", Value: "c d", Enabled: true}},
		},
		{
			name:    "不正なエスケープは生文字列のまま採用しエラーにしない",
			content: "k=%zz",
			want:    []FormRow{{Key: "k", Value: "%zz", Enabled: true}},
		},
		{
			name:    "空要素は読み飛ばす",
			content: "a=1&&b=2",
			want: []FormRow{
				{Key: "a", Value: "1", Enabled: true},
				{Key: "b", Value: "2", Enabled: true},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseFormPairs(tc.content)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseFormPairs(%q) = %+v, want %+v", tc.content, got, tc.want)
			}
		})
	}
}

func TestEncodeFormPairs(t *testing.T) {
	tests := []struct {
		name  string
		pairs []FormRow
		want  string
	}{
		{
			name:  "行なしは空文字列",
			pairs: nil,
			want:  "",
		},
		{
			name: "入力順を保つ（キー名でソートしない）",
			pairs: []FormRow{
				{Key: "z", Value: "1", Enabled: true},
				{Key: "a", Value: "2", Enabled: true},
			},
			want: "z=1&a=2",
		},
		{
			name: "無効行を除外する",
			pairs: []FormRow{
				{Key: "a", Value: "1", Enabled: true},
				{Key: "b", Value: "2", Enabled: false},
				{Key: "c", Value: "3", Enabled: true},
			},
			want: "a=1&c=3",
		},
		{
			name: "空キー行を除外する（編集中の未入力行を送らない）",
			pairs: []FormRow{
				{Key: "", Value: "orphan", Enabled: true},
				{Key: "a", Value: "1", Enabled: true},
			},
			want: "a=1",
		},
		{
			name:  "特殊文字をエスケープする",
			pairs: []FormRow{{Key: "a b", Value: "c&d=e", Enabled: true}},
			want:  "a+b=c%26d%3De",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := EncodeFormPairs(tc.pairs); got != tc.want {
				t.Errorf("EncodeFormPairs(%+v) = %q, want %q", tc.pairs, got, tc.want)
			}
		})
	}
}

// TestEncodeFormPairs_RoundTrip は Encode → Parse で行が保たれることを確認する。
func TestEncodeFormPairs_RoundTrip(t *testing.T) {
	pairs := []FormRow{
		{Key: "z", Value: "a b", Enabled: true},
		{Key: "a", Value: "c&d", Enabled: true},
		{Key: "%weird", Value: "", Enabled: true},
	}
	got := ParseFormPairs(EncodeFormPairs(pairs))
	if !reflect.DeepEqual(got, pairs) {
		t.Errorf("round trip = %+v, want %+v", got, pairs)
	}
}

func TestRequestBody_NormalizeForms(t *testing.T) {
	t.Run("旧データの文字列から行を復元する", func(t *testing.T) {
		b := RequestBody{
			Type:     BodyTypeFormURLEncoded,
			Contents: map[string]string{BodyTypeFormURLEncoded: "a=1&b=2"},
		}
		b.NormalizeForms()

		want := []FormRow{
			{Key: "a", Value: "1", Enabled: true},
			{Key: "b", Value: "2", Enabled: true},
		}
		if !reflect.DeepEqual(b.FormURLEncoded, want) {
			t.Errorf("FormURLEncoded = %+v, want %+v", b.FormURLEncoded, want)
		}
	})

	t.Run("移行後は Contents の form 系キーを削除して正を一本化する", func(t *testing.T) {
		b := RequestBody{
			Type: BodyTypeFormData,
			Contents: map[string]string{
				BodyTypeFormData: "a=1",
				"json":           `{"k":"v"}`,
			},
		}
		b.NormalizeForms()

		if _, ok := b.Contents[BodyTypeFormData]; ok {
			t.Error("Contents に form-data キーが残っている（旧文字列から行が復活する）")
		}
		if b.Contents["json"] != `{"k":"v"}` {
			t.Errorf("form 系以外の Contents を消してはいけない: %+v", b.Contents)
		}
	})

	t.Run("既に行があれば上書きしない", func(t *testing.T) {
		existing := []FormRow{{Key: "kept", Value: "v", Enabled: false}}
		b := RequestBody{
			Type:     BodyTypeFormData,
			FormData: existing,
			Contents: map[string]string{BodyTypeFormData: "stale=1"},
		}
		b.NormalizeForms()

		if !reflect.DeepEqual(b.FormData, existing) {
			t.Errorf("FormData = %+v, want %+v", b.FormData, existing)
		}
	})

	// 全行を削除した状態は omitempty で JSON から消え、読み込み時 nil になる。
	// このとき旧文字列が残っていると行が復活してしまうため、キー削除で防いでいる。
	t.Run("全行削除後に再正規化しても行は復活しない", func(t *testing.T) {
		b := RequestBody{
			Type:     BodyTypeFormData,
			Contents: map[string]string{BodyTypeFormData: "a=1"},
		}
		b.NormalizeForms() // 旧データ移行
		b.FormData = nil   // ユーザーが全行削除
		b.NormalizeForms() // 保存 → 再読み込み相当

		if b.FormData != nil {
			t.Errorf("FormData = %+v, want nil（削除した行が復活している）", b.FormData)
		}
	})

	t.Run("Contents が nil でも落ちない", func(t *testing.T) {
		b := RequestBody{Type: BodyTypeFormData}
		b.NormalizeForms()

		if b.FormData != nil {
			t.Errorf("FormData = %+v, want nil", b.FormData)
		}
	})

	t.Run("両方の form 種別を独立に移行する", func(t *testing.T) {
		b := RequestBody{
			Type: BodyTypeFormData,
			Contents: map[string]string{
				BodyTypeFormData:       "a=1",
				BodyTypeFormURLEncoded: "b=2",
			},
		}
		b.NormalizeForms()

		if len(b.FormData) != 1 || b.FormData[0].Key != "a" {
			t.Errorf("FormData = %+v", b.FormData)
		}
		if len(b.FormURLEncoded) != 1 || b.FormURLEncoded[0].Key != "b" {
			t.Errorf("FormURLEncoded = %+v", b.FormURLEncoded)
		}
	})
}

func TestRequestBody_FormPairs(t *testing.T) {
	pairs := []FormRow{{Key: "a", Value: "1", Enabled: true}}

	t.Run("Type に対応する行を返す", func(t *testing.T) {
		b := RequestBody{Type: BodyTypeFormData, FormData: pairs}
		if !reflect.DeepEqual(b.FormPairs(), pairs) {
			t.Errorf("FormPairs() = %+v, want %+v", b.FormPairs(), pairs)
		}
	})

	t.Run("form 系以外では nil", func(t *testing.T) {
		b := RequestBody{Type: "json", FormData: pairs}
		if b.FormPairs() != nil {
			t.Errorf("FormPairs() = %+v, want nil", b.FormPairs())
		}
	})
}

func TestFormRow_EffectiveKind(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want string
	}{
		{name: "未設定は text とみなす（Kind 導入前の行）", kind: "", want: FormRowKindText},
		{name: "text はそのまま", kind: FormRowKindText, want: FormRowKindText},
		{name: "json はそのまま", kind: FormRowKindJSON, want: FormRowKindJSON},
		{name: "file はそのまま", kind: FormRowKindFile, want: FormRowKindFile},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := FormRow{Kind: tc.kind}
			if got := r.EffectiveKind(); got != tc.want {
				t.Errorf("EffectiveKind() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Kind 導入前に保存された行は kind/filePath/contentType を持たない。
// 読み込みで落ちず text 行として扱えることを保証する。
func TestFormRow_UnmarshalLegacyJSON(t *testing.T) {
	var b RequestBody
	legacy := `{"type":"form-data","contents":{},"formData":[{"key":"a","value":"1","enabled":true}]}`
	if err := json.Unmarshal([]byte(legacy), &b); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := []FormRow{{Key: "a", Value: "1", Enabled: true}}
	if !reflect.DeepEqual(b.FormData, want) {
		t.Errorf("FormData = %+v, want %+v", b.FormData, want)
	}
	if got := b.FormData[0].EffectiveKind(); got != FormRowKindText {
		t.Errorf("EffectiveKind() = %q, want %q", got, FormRowKindText)
	}
}

// 未設定の kind/filePath/contentType は omitempty で保存内容を増やさない。
func TestFormRow_MarshalOmitsUnsetFields(t *testing.T) {
	row := FormRow{Key: "a", Value: "1", Enabled: true}
	got, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	want := `{"key":"a","value":"1","enabled":true}`
	if string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

// urlencoded のワイヤ形式に載るのは Key/Value だけ。json 行の値はそのまま
// エスケープされ、kind や contentType は出力に影響しない。
func TestEncodeFormPairs_IgnoresKind(t *testing.T) {
	pairs := []FormRow{
		{Key: "a", Value: "1", Kind: FormRowKindText, Enabled: true},
		{Key: "j", Value: `{"k":"v"}`, Kind: FormRowKindJSON, ContentType: "application/json", Enabled: true},
	}
	want := `a=1&j=%7B%22k%22%3A%22v%22%7D`
	if got := EncodeFormPairs(pairs); got != want {
		t.Errorf("EncodeFormPairs() = %q, want %q", got, want)
	}
}

func TestGuessFileContentType(t *testing.T) {
	t.Run("判定できない拡張子はバイナリ扱い", func(t *testing.T) {
		if got := GuessFileContentType("a.unknown-ext-xyz"); got != "application/octet-stream" {
			t.Errorf("GuessFileContentType() = %q, want application/octet-stream", got)
		}
	})

	t.Run("拡張子が無い場合もバイナリ扱い", func(t *testing.T) {
		if got := GuessFileContentType("noext"); got != "application/octet-stream" {
			t.Errorf("GuessFileContentType() = %q, want application/octet-stream", got)
		}
	})

	// 具体的な MIME は OS 依存（Windows はレジストリ参照）なので、
	// 判定できた場合に octet-stream へ落ちないことだけを確認する。
	t.Run("既知の拡張子はバイナリ扱いに落とさない", func(t *testing.T) {
		if got := GuessFileContentType("a.json"); got == "application/octet-stream" {
			t.Errorf("GuessFileContentType(a.json) = %q, want a json media type", got)
		}
	})
}
