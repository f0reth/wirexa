// Command gen-events は internal/domain の Wails イベント名定数から
// フロントエンド用の TypeScript 定数ファイルを生成する。
// internal/domain/events.go の go:generate ディレクティブから呼ばれる。
//
// 生成物: frontend/src/shared/wails-events.ts
// Go 側の定数値がフロントへ機械的に伝播し、イベント名の二重宣言・乖離を防ぐ。
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/f0reth/Wirexa/internal/domain"
)

// eventPair は TS 側のキー名と Go の定数値を対応づける。
// Go 定数を直接参照するため、値の変更は再生成でそのまま反映される。
type eventPair struct {
	Key   string
	Value string
}

var events = []eventPair{
	{"mqttConnected", domain.EventMQTTConnected},
	{"mqttDisconnected", domain.EventMQTTDisconnected},
	{"mqttConnectionLost", domain.EventMQTTConnectionLost},
	{"mqttConnectionFailed", domain.EventMQTTConnectionFailed},
	{"mqttMessage", domain.EventMQTTMessage},
	{"udpMessage", domain.EventUDPMessage},
	{"appBeforeClose", domain.EventAppBeforeClose},
}

func main() {
	out := flag.String("out", "../../frontend/src/shared/wails-events.ts",
		"生成先の TypeScript ファイルパス (go generate 実行ディレクトリからの相対)")
	flag.Parse()

	var b strings.Builder
	b.WriteString("// Code generated from internal/domain/events.go by tools/gen-events; DO NOT EDIT.\n\n")
	b.WriteString("export const WailsEvents = {\n")
	for _, e := range events {
		fmt.Fprintf(&b, "  %s: %q,\n", e.Key, e.Value)
	}
	b.WriteString("} as const;\n\n")

	// MQTT イベント (値が "mqtt:" 始まり) をユニオン型として書き出す。
	b.WriteString("export type MqttEventName =\n")
	mqtt := make([]string, 0, len(events))
	for _, e := range events {
		if strings.HasPrefix(e.Value, "mqtt:") {
			mqtt = append(mqtt, e.Value)
		}
	}
	for i, v := range mqtt {
		term := ""
		if i == len(mqtt)-1 {
			term = ";"
		}
		fmt.Fprintf(&b, "  | %q%s\n", v, term)
	}

	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil { //nolint:gosec // 生成物は公開ソース、0644 で問題ない
		fmt.Fprintf(os.Stderr, "gen-events: write %s: %v\n", *out, err)
		os.Exit(1)
	}
}
