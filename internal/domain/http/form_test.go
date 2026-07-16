package httpdomain

import (
	"reflect"
	"testing"
)

func TestParseFormPairs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []KeyValuePair
	}{
		{
			name:    "空文字列は行なし",
			content: "",
			want:    nil,
		},
		{
			name:    "行順を保つ（url.Values と違いソートしない）",
			content: "z=1&a=2&m=3",
			want: []KeyValuePair{
				{Key: "z", Value: "1", Enabled: true},
				{Key: "a", Value: "2", Enabled: true},
				{Key: "m", Value: "3", Enabled: true},
			},
		},
		{
			name:    "重複キーを潰さない",
			content: "k=1&k=2",
			want: []KeyValuePair{
				{Key: "k", Value: "1", Enabled: true},
				{Key: "k", Value: "2", Enabled: true},
			},
		},
		{
			name:    "値に = を含む",
			content: "k=a=b",
			want:    []KeyValuePair{{Key: "k", Value: "a=b", Enabled: true}},
		},
		{
			name:    "= が無い要素は値を空にする",
			content: "k",
			want:    []KeyValuePair{{Key: "k", Value: "", Enabled: true}},
		},
		{
			name:    "パーセントエンコードと + をデコードする",
			content: "a%20b=c+d",
			want:    []KeyValuePair{{Key: "a b", Value: "c d", Enabled: true}},
		},
		{
			name:    "不正なエスケープは生文字列のまま採用しエラーにしない",
			content: "k=%zz",
			want:    []KeyValuePair{{Key: "k", Value: "%zz", Enabled: true}},
		},
		{
			name:    "空要素は読み飛ばす",
			content: "a=1&&b=2",
			want: []KeyValuePair{
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
		pairs []KeyValuePair
		want  string
	}{
		{
			name:  "行なしは空文字列",
			pairs: nil,
			want:  "",
		},
		{
			name: "入力順を保つ（キー名でソートしない）",
			pairs: []KeyValuePair{
				{Key: "z", Value: "1", Enabled: true},
				{Key: "a", Value: "2", Enabled: true},
			},
			want: "z=1&a=2",
		},
		{
			name: "無効行を除外する",
			pairs: []KeyValuePair{
				{Key: "a", Value: "1", Enabled: true},
				{Key: "b", Value: "2", Enabled: false},
				{Key: "c", Value: "3", Enabled: true},
			},
			want: "a=1&c=3",
		},
		{
			name: "空キー行を除外する（編集中の未入力行を送らない）",
			pairs: []KeyValuePair{
				{Key: "", Value: "orphan", Enabled: true},
				{Key: "a", Value: "1", Enabled: true},
			},
			want: "a=1",
		},
		{
			name:  "特殊文字をエスケープする",
			pairs: []KeyValuePair{{Key: "a b", Value: "c&d=e", Enabled: true}},
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
	pairs := []KeyValuePair{
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

		want := []KeyValuePair{
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
		existing := []KeyValuePair{{Key: "kept", Value: "v", Enabled: false}}
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
	pairs := []KeyValuePair{{Key: "a", Value: "1", Enabled: true}}

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
