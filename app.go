// Package main is the entry point for the Wirexa desktop application.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/f0reth/Wirexa/internal/adapters"
	httpapp "github.com/f0reth/Wirexa/internal/application/http"
	mqttapp "github.com/f0reth/Wirexa/internal/application/mqtt"
	udpapp "github.com/f0reth/Wirexa/internal/application/udp"
	cmn "github.com/f0reth/Wirexa/internal/domain"
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
	_ udpdomain.TargetRepository      = (*infra.JSONStore[udpdomain.UDPTarget])(nil)
)

const wirexaConfigDir = "Wirexa"

// ウィンドウの既定・最小サイズ。main.go の options.App と共有する。
const (
	defaultWindowWidth  = 1280
	defaultWindowHeight = 800
	minWindowWidth      = 800
	minWindowHeight     = 600
)

type App struct {
	ctx            context.Context
	mqttHandler    *adapters.MQTTHandler
	httpHandler    *adapters.HTTPHandler
	udpHandler     *adapters.UDPHandler
	logHandler     *adapters.LogHandler
	openAPIHandler *adapters.OpenAPIHandler
	netClient      *httpinfra.NetClient
	windowMgr      *infra.WindowManager
	ready          bool
	quitConfirmed  bool
}

func NewApp() *App {
	return &App{
		mqttHandler:    &adapters.MQTTHandler{},
		httpHandler:    &adapters.HTTPHandler{},
		udpHandler:     &adapters.UDPHandler{},
		logHandler:     &adapters.LogHandler{},
		openAPIHandler: &adapters.OpenAPIHandler{},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.initialize(ctx); err != nil {
		// GUI アプリではコンソールが無いため、致命的エラーはダイアログで提示してから終了する。
		log.Printf("startup failed: %v", err) // stderr へのベストエフォート
		_, dialogErr := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "Wirexa - Startup Failed",
			Message: fmt.Sprintf("The application could not be started.\n\n%v", err),
		})
		if dialogErr != nil {
			log.Printf("failed to show startup error dialog: %v", dialogErr)
		}
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
		return fmt.Errorf("failed to get config directory: %w", err)
	}
	a.windowMgr = infra.NewWindowManager(
		ctx,
		filepath.Join(configDir, wirexaConfigDir, "window-state.json"),
		minWindowWidth, minWindowHeight,
	)

	// 前回セッションで残った打ち切りレスポンスの一時ファイルを掃除する。
	// startup 済 (= 単一インスタンスロックを通過したプライマリ) なので全削除して安全。
	httpinfra.SweepStaleTempFiles()

	logger, err := infra.NewFileLogger(filepath.Join(configDir, wirexaConfigDir, "logs"))
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	adapters.SetupLogHandler(a.logHandler, logger)

	// MQTT / UDP の両サービスで共有する (ctx を保持するだけのステートレスな型)。
	emitter := infra.NewWailsEmitter(ctx)

	clientFactory := mqttinfra.NewPahoClientFactory(mqttinfra.MQTTClientConfig{})
	mqttSvc := mqttapp.NewMQTTService(emitter, clientFactory, logger)

	profileRepo, err := infra.NewJSONStore(
		filepath.Join(configDir, wirexaConfigDir, "mqtt-profiles"),
		func(p *mqttdomain.BrokerProfile) string { return p.ID },
	)
	if err != nil {
		return fmt.Errorf("failed to create MQTT profile store: %w", err)
	}
	profileRepo.SetLogger(logger)
	profileSvc, err := mqttapp.NewProfileService(profileRepo)
	if err != nil {
		return fmt.Errorf("failed to load MQTT profiles: %w", err)
	}
	adapters.SetupMQTTHandler(a.mqttHandler, mqttSvc, profileSvc)

	collRepo, err := infra.NewJSONStore(
		filepath.Join(configDir, wirexaConfigDir, "collections"),
		func(c *httpdomain.Collection) string { return c.ID },
	)
	if err != nil {
		return fmt.Errorf("failed to create collection store: %w", err)
	}
	collRepo.SetLogger(logger)
	layoutRepo := httpinfra.NewSidebarLayoutRepository(filepath.Join(configDir, wirexaConfigDir, "sidebar_layout.json"))
	collSvc, err := httpapp.NewCollectionService(collRepo, layoutRepo)
	if err != nil {
		return fmt.Errorf("failed to initialize collections: %w", err)
	}
	a.netClient = httpinfra.NewNetClient()
	reqSvc := httpapp.NewHTTPRequestService(a.netClient, logger)
	adapters.SetupHTTPHandler(ctx, a.httpHandler, reqSvc, collSvc, collSvc, a.netClient)

	targetRepo, err := infra.NewJSONStore(
		filepath.Join(configDir, wirexaConfigDir, "udp-targets"),
		func(t *udpdomain.UDPTarget) string { return t.ID },
	)
	if err != nil {
		return fmt.Errorf("failed to create UDP target store: %w", err)
	}
	targetRepo.SetLogger(logger)
	targetSvc, err := udpapp.NewTargetService(targetRepo)
	if err != nil {
		return fmt.Errorf("failed to load UDP targets: %w", err)
	}
	udpSocket := udpinfra.NewNetSocket()
	sendSvc := udpapp.NewUDPSendService(udpSocket, logger)
	listenSvc := udpapp.NewUDPListenerService(udpSocket, emitter, logger)
	adapters.SetupUDPHandler(a.udpHandler, sendSvc, targetSvc, listenSvc)

	adapters.SetupOpenAPIHandler(ctx, a.openAPIHandler,
		filepath.Join(configDir, wirexaConfigDir, "openapi-recents.json"), logger)

	a.windowMgr.Restore()

	return nil
}

// onSecondInstanceLaunch は二重起動時に既存インスタンス側で呼ばれ、ウィンドウを前面化する。
func (a *App) onSecondInstanceLaunch(_ options.SecondInstanceData) {
	if a.ctx == nil {
		return // startup 完了前は何もしない
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.Show(a.ctx)
}

// beforeClose はウィンドウを閉じようとしたときに呼ばれる。
// true を返すと閉じるのを阻止する。未保存文書の有無はフロントエンドしか
// 知らないため、"app:before-close" を通知して一旦阻止し、フロント側の判断
// (ConfirmQuit の呼び出し) を待つ。ConfirmQuit 経由で quitConfirmed が立てば
// 次の呼び出しでそのまま閉じる。
func (a *App) beforeClose(ctx context.Context) bool {
	// ウィンドウが生存しているこの時点で状態を保存する (冪等なので複数回呼ばれてよい)。
	if a.windowMgr != nil {
		a.windowMgr.Save()
	}
	if a.quitConfirmed {
		return false
	}
	runtime.EventsEmit(ctx, cmn.EventAppBeforeClose)
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
	if a.netClient != nil {
		a.netClient.Cleanup()
	}
}
