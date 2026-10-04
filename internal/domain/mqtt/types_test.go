package mqttdomain

import (
	"encoding/json"
	"maps"
	"testing"
)

// marshalToMap は v を JSON 化し、フロントエンドが受け取るのと同じキーと値の組にして返す。
func marshalToMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal(%#v): %v", v, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal(%s): %v", b, err)
	}
	return m
}

// ライフサイクルイベントの配線形式のキーを固定する。
// テストのエミッターは Go の値をそのまま記録して JSON 化を通さないので、json タグはここで守る。
func TestLifecycleEvents_WireFormat(t *testing.T) {
	tests := []struct {
		name  string
		event any
		want  map[string]any
	}{
		{
			name:  "ConnectionEvent",
			event: ConnectionEvent{ConnectionID: "c1"},
			want:  map[string]any{"connectionId": "c1"},
		},
		{
			name:  "ConnectionErrorEvent",
			event: ConnectionErrorEvent{ConnectionID: "c1", Error: "broker went away"},
			want:  map[string]any{"connectionId": "c1", "error": "broker went away"},
		},
		{
			name:  "ConnectionErrorEvent の空のエラー文言でも error キーを残す",
			event: ConnectionErrorEvent{ConnectionID: "c1"},
			want:  map[string]any{"connectionId": "c1", "error": ""},
		},
		{
			name:  "SubscriptionDropped",
			event: SubscriptionDropped{ConnectionID: "c1", Topic: "a/#", Error: "subscription rejected"},
			want:  map[string]any{"connectionId": "c1", "topic": "a/#", "error": "subscription rejected"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := marshalToMap(t, tc.event); !maps.Equal(got, tc.want) {
				t.Errorf("json = %v, want %v", got, tc.want)
			}
		})
	}
}
