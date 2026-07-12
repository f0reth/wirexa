// Package main is the entry point for the Wirexa desktop application.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/f0reth/Wirexa/internal/adapters"
	httpapp "github.com/f0reth/Wirexa/internal/application/http"
	mqttapp "github.com/f0reth/Wirexa/internal/application/mqtt"
	udpapp "github.com/f0reth/Wirexa/internal/application/udp"
	httpdomain "github.com/f0reth/Wirexa/internal/domain/http"
	mqttdomain "github.com/f0reth/Wirexa/internal/domain/mqtt"
	udpdomain "github.com/f0reth/Wirexa/internal/domain/udp"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
	httpinfra "github.com/f0reth/Wirexa/internal/infrastructure/http"
	mqttinfra "github.com/f0reth/Wirexa/internal/infrastructure/mqtt"
	udpinfra "github.com/f0reth/Wirexa/internal/infrastructure/udp"
)

// コンパイル時に各ドメインインターフェースを JSONStore[T] が満たすことを検証
var (
	_ httpdomain.CollectionRepository = (*infra.JSONStore[httpdomain.Collection])(nil)
	_ mqttdomain.ProfileRepository    = (*infra.JSONStore[mqttdomain.BrokerProfile])(nil)
	_ udpdomain.TargetRepository      = (*infra.JSONStore[udpdomain.UdpTarget])(nil)
)

const wirexaConfigDir = "Wirexa"

type App struct {
	mqttHandler    *adapters.MqttHandler
	httpHandler    *adapters.HttpHandler
	udpHandler     *adapters.UdpHandler
	logHandler     *adapters.LogHandler
	openAPIHandler *adapters.OpenAPIHandler
	ctx            context.Context
	ready          bool
	quitConfirmed  bool
}

func NewApp() *App {
	return &App{
		mqttHandler:    &adapters.MqttHandler{},
		httpHandler:    &adapters.HttpHandler{},
		udpHandler:     &adapters.UdpHandler{},
		logHandler:     &adapters.LogHandler{},
		openAPIHandler: &adapters.OpenAPIHandler{},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.initialize(ctx); err != nil {
		// GUI アプリではコンソールが無いため、致命的エラーはダイアログで提示してから終了する。
		log.Printf("startup failed: %v", err) // stderr へのベストエフォート
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "Wirexa - 起動に失敗しました",
			Message: fmt.Sprintf("アプリケーションを起動できませんでした。\n\n%v", err),
		})
		runtime.Quit(ctx)
		return
	}
	a.ready = true
}

// initialize は各サービスの構築を行い、失敗時はエラーを返す。
// 破損した JSON ファイルは JSONStore.Load 側で退避・スキップされるため、
// ここでの失敗は設定ディレクトリが作れない等の継続不能なケースに限られる。
func (a *App) initialize(ctx context.Context) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("設定ディレクトリの取得に失敗しました: %w", err)
	}

	logger, err := infra.NewFileLogger(filepath.Join(configDir, wirexaConfigDir, "logs"))
	if err != nil {
		return fmt.Errorf("ロガーの初期化に失敗しました: %w", err)
	}
	adapters.SetupLogHandler(a.logHandler, logger)

	emitter := infra.NewWailsEmitter(ctx)
	clientFactory := mqttinfra.NewPahoClientFactory(mqttinfra.MqttClientConfig{})
	mqttSvc := mqttapp.NewMqttService(emitter, clientFactory, logger)

	profileRepo, err := infra.NewJSONStore(
		filepath.Join(configDir, wirexaConfigDir, "mqtt-profiles"),
		func(p *mqttdomain.BrokerProfile) string { return p.ID },
	)
	if err != nil {
		return fmt.Errorf("MQTT プロファイルの保存先を作成できませんでした: %w", err)
	}
	profileRepo.SetLogger(logger)
	profileSvc, err := mqttapp.NewProfileService(profileRepo)
	if err != nil {
		return fmt.Errorf("MQTT プロファイルの読み込みに失敗しました: %w", err)
	}
	adapters.SetupMqttHandler(a.mqttHandler, mqttSvc, profileSvc)

	collRepo, err := infra.NewJSONStore(
		filepath.Join(configDir, wirexaConfigDir, "collections"),
		func(c *httpdomain.Collection) string { return c.ID },
	)
	if err != nil {
		return fmt.Errorf("コレクションの保存先を作成できませんでした: %w", err)
	}
	collRepo.SetLogger(logger)
	layoutRepo := httpinfra.NewSidebarLayoutRepository(filepath.Join(configDir, wirexaConfigDir, "sidebar_layout.json"))
	collSvc, err := httpapp.NewCollectionService(collRepo, layoutRepo)
	if err != nil {
		return fmt.Errorf("コレクションの初期化に失敗しました: %w", err)
	}
	netClient := httpinfra.NewNetClient()
	reqSvc := httpapp.NewHTTPRequestService(netClient, logger)
	adapters.SetupHTTPHandler(ctx, a.httpHandler, reqSvc, collSvc, collSvc, netClient)

	targetRepo, err := infra.NewJSONStore(
		filepath.Join(configDir, wirexaConfigDir, "udp-targets"),
		func(t *udpdomain.UdpTarget) string { return t.ID },
	)
	if err != nil {
		return fmt.Errorf("UDP ターゲットの保存先を作成できませんでした: %w", err)
	}
	targetRepo.SetLogger(logger)
	targetSvc, err := udpapp.NewTargetService(targetRepo)
	if err != nil {
		return fmt.Errorf("UDP ターゲットの読み込みに失敗しました: %w", err)
	}
	udpSocket := udpinfra.NewNetSocket()
	sendSvc := udpapp.NewUdpSendService(udpSocket, logger)
	udpEmitter := infra.NewWailsEmitter(ctx)
	listenSvc := udpapp.NewUdpListenerService(udpSocket, udpEmitter, logger)
	adapters.SetupUdpHandler(a.udpHandler, sendSvc, targetSvc, listenSvc)

	adapters.SetupOpenAPIHandler(ctx, a.openAPIHandler,
		filepath.Join(configDir, wirexaConfigDir, "openapi-recents.json"))

	return nil
}

// beforeClose はウィンドウを閉じようとしたときに呼ばれる。
// true を返すと閉じるのを阻止する。未保存文書の有無はフロントエンドしか
// 知らないため、"app:before-close" を通知して一旦阻止し、フロント側の判断
// (ConfirmQuit の呼び出し) を待つ。ConfirmQuit 経由で quitConfirmed が立てば
// 次の呼び出しでそのまま閉じる。
func (a *App) beforeClose(ctx context.Context) bool {
	if a.quitConfirmed {
		return false
	}
	runtime.EventsEmit(ctx, "app:before-close")
	return true
}

// ConfirmQuit はフロントエンドが「終了してよい」と判断したときに呼ぶ RPC。
// quitConfirmed を立ててから終了させることで beforeClose を通過させる。
func (a *App) ConfirmQuit() {
	a.quitConfirmed = true
	runtime.Quit(a.ctx)
}

func (a *App) shutdown(_ context.Context) {
	// 初期化に失敗した場合はハンドラーのサービスが未設定 (nil) なので何もしない。
	if !a.ready {
		return
	}
	a.mqttHandler.Shutdown()
	a.udpHandler.Shutdown()
}
