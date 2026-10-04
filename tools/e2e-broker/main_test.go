package main

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
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

// getClients は GET /clients の結果を返す。
func getClients(t *testing.T, url string) []clientInfo {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url+"/clients", http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /clients: %v", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			t.Errorf("close body: %v", err)
		}
	}()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /clients: status = %d, want %d", res.StatusCode, http.StatusOK)
	}
	var clients []clientInfo
	if err := json.NewDecoder(res.Body).Decode(&clients); err != nil {
		t.Fatalf("decode /clients: %v", err)
	}
	return clients
}

// waitToken は paho の操作の完了を待ち、失敗していたらテストを止める。
func waitToken(t *testing.T, action string, token paho.Token) {
	t.Helper()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatalf("%s: timed out", action)
	}
	if err := token.Error(); err != nil {
		t.Fatalf("%s: %v", action, err)
	}
}

func TestControl_Clients(t *testing.T) {
	server := mqtt.New(&mqtt.Options{InlineClient: true})
	hook := newDenyHook()
	if err := server.AddHook(hook, nil); err != nil {
		t.Fatalf("AddHook: %v", err)
	}
	tcp := listeners.NewTCP(listeners.Config{ID: "e2e", Address: "127.0.0.1:0"})
	if err := server.AddListener(tcp); err != nil {
		t.Fatalf("AddListener: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	ts := httptest.NewServer(newControlHandler(server, hook))
	t.Cleanup(ts.Close)

	// インラインクライアントは数えない。空のときも null ではなく空の配列を返す。
	if got := getClients(t, ts.URL); got == nil || len(got) != 0 {
		t.Fatalf("clients before connecting = %#v, want an empty array", got)
	}

	client := paho.NewClient(paho.NewClientOptions().
		AddBroker("tcp://" + tcp.Address()).
		SetClientID("client-a").
		SetUsername("alice").
		SetPassword("s3cret").
		SetAutoReconnect(false))
	waitToken(t, "connect", client.Connect())
	t.Cleanup(func() { client.Disconnect(0) })
	waitToken(t, "subscribe b/#", client.Subscribe("b/#", 1, nil))
	waitToken(t, "subscribe a/topic", client.Subscribe("a/topic", 0, nil))

	want := []clientInfo{{
		ID:       "client-a",
		Username: "alice",
		Password: "s3cret",
		Listener: "e2e",
		// 購読はフィルターの順に並ぶ。
		Subscriptions: []subscriptionInfo{{Filter: "a/topic", QoS: 0}, {Filter: "b/#", QoS: 1}},
	}}
	if got := getClients(t, ts.URL); !reflect.DeepEqual(got, want) {
		t.Errorf("clients after subscribing = %#v, want %#v", got, want)
	}

	waitToken(t, "unsubscribe b/#", client.Unsubscribe("b/#"))
	want[0].Subscriptions = []subscriptionInfo{{Filter: "a/topic", QoS: 0}}
	if got := getClients(t, ts.URL); !reflect.DeepEqual(got, want) {
		t.Errorf("clients after unsubscribing = %#v, want %#v", got, want)
	}

	// 切断したクライアントは返さない。ブローカーが切断を処理するまで待つ。
	client.Disconnect(0)
	deadline := time.Now().Add(5 * time.Second)
	for len(getClients(t, ts.URL)) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("clients after disconnecting = %#v, want none", getClients(t, ts.URL))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// アプリは証明書を検証するので、ここで作る自己署名の証明書では接続できない。その前提として、
// 証明書そのものは 127.0.0.1 向けのサーバー証明書として正しいことを確かめる。
func TestSelfSignedTLSConfig(t *testing.T) {
	config, err := selfSignedTLSConfig()
	if err != nil {
		t.Fatalf("selfSignedTLSConfig: %v", err)
	}
	if len(config.Certificates) != 1 {
		t.Fatalf("certificates = %d, want 1", len(config.Certificates))
	}
	cert, err := x509.ParseCertificate(config.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	// 自身を信頼するルートにすれば検証を通る (システムの証明書ストアには無い)。
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	for _, name := range []string{"127.0.0.1", "localhost"} {
		if _, err := cert.Verify(x509.VerifyOptions{DNSName: name, Roots: roots}); err != nil {
			t.Errorf("Verify(%q) with the certificate as its own root: %v", name, err)
		}
	}
	if _, err := cert.Verify(x509.VerifyOptions{DNSName: "127.0.0.1", Roots: x509.NewCertPool()}); err == nil {
		t.Error("Verify without the certificate as a root should fail")
	}
}
