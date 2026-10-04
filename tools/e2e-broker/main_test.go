package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
)

func TestDenyHook_DeniesOnlyRegisteredSubscriptions(t *testing.T) {
	hook := newDenyHook()
	if !hook.OnACLCheck(nil, "a/#", false) {
		t.Fatal("subscription should be allowed before any filter is denied")
	}

	hook.deny("a/#")

	if hook.OnACLCheck(nil, "a/#", false) {
		t.Error("subscription to the denied filter should be rejected")
	}
	// フィルターの文字列が同じ購読だけを拒否する (トピックとしての一致は見ない)。
	for _, filter := range []string{"a/b", "b/#", "#"} {
		if !hook.OnACLCheck(nil, filter, false) {
			t.Errorf("subscription to %q should be allowed", filter)
		}
	}
	if !hook.OnACLCheck(nil, "a/#", true) {
		t.Error("publish should be allowed even to a denied filter")
	}

	hook.allowAll()

	if !hook.OnACLCheck(nil, "a/#", false) {
		t.Error("subscription should be allowed again after allowAll")
	}
}

// 判定はブローカーの接続ごとの goroutine から、登録と解除は操作用 HTTP から同時に呼ばれる。
// 競合は task go:test:race で検出する。
func TestDenyHook_ConcurrentUse(t *testing.T) {
	hook := newDenyHook()
	var wg sync.WaitGroup
	for i := range 8 {
		filter := fmt.Sprintf("topic/%d", i)
		wg.Go(func() {
			for range 200 {
				hook.deny(filter)
				hook.OnACLCheck(nil, filter, false)
				hook.allowAll()
			}
		})
	}
	wg.Wait()

	if !hook.OnACLCheck(nil, "topic/0", false) {
		t.Error("no filter should remain denied after every goroutine called allowAll")
	}
}

func newTestControl(t *testing.T) (*httptest.Server, *denyHook) {
	t.Helper()
	server := mqtt.New(&mqtt.Options{InlineClient: true})
	hook := newDenyHook()
	if err := server.AddHook(hook, nil); err != nil {
		t.Fatalf("AddHook: %v", err)
	}
	ts := httptest.NewServer(newControlHandler(server, hook))
	t.Cleanup(ts.Close)
	return ts, hook
}

func request(t *testing.T, method, url string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	if err := res.Body.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}
	return res.StatusCode
}

func TestControl_Deny(t *testing.T) {
	ts, hook := newTestControl(t)

	if got := request(t, http.MethodPost, ts.URL+"/deny"); got != http.StatusBadRequest {
		t.Errorf("POST /deny without filter: status = %d, want %d", got, http.StatusBadRequest)
	}

	// フィルターの # はクエリの中でエスケープして渡す。
	if got := request(t, http.MethodPost, ts.URL+"/deny?filter=a%2F%23"); got != http.StatusNoContent {
		t.Fatalf("POST /deny: status = %d, want %d", got, http.StatusNoContent)
	}
	if hook.OnACLCheck(nil, "a/#", false) {
		t.Error("subscription to the denied filter should be rejected")
	}

	if got := request(t, http.MethodDelete, ts.URL+"/deny"); got != http.StatusNoContent {
		t.Fatalf("DELETE /deny: status = %d, want %d", got, http.StatusNoContent)
	}
	if !hook.OnACLCheck(nil, "a/#", false) {
		t.Error("subscription should be allowed after DELETE /deny")
	}
}

func TestControl_DisconnectClientsWithoutClients(t *testing.T) {
	ts, _ := newTestControl(t)

	if got := request(t, http.MethodPost, ts.URL+"/disconnect-clients"); got != http.StatusNoContent {
		t.Errorf("POST /disconnect-clients: status = %d, want %d", got, http.StatusNoContent)
	}
}
