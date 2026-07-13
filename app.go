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

// ウィンドウの既定・最小サイズ。main.go の options.App と共有する。
const (
	defaultWindowWidth  = 1280
	defaultWindowHeight = 800
	minWindowWidth      = 800
	minWindowHeight     = 600
)

type App struct {
	mqttHandler    *adapters.MqttHandler
	httpHandler    *adapters.HttpHandler
	udpHandler     *adapters.UdpHandler
	logHandler     *adapters.LogHandler
	openAPIHandler *adapters.OpenAPIHandler
	netClient      *httpinfra.NetClient
	ctx            context.Context
	ready          bool
	quitConfirmed  bool

	windowStatePath string
	// winState は最後に把握した通常時 (非最大化) のウィンドウバウンズ。
	winState infra.WindowState
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
	a.windowStatePath = filepath.Join(configDir, wirexaConfigDir, "window-state.json")

	// 前回セッションで残った打ち切りレスポンスの一時ファイルを掃除する。
	// startup 済 (= 単一インスタンスロックを通過したプライマリ) なので全削除して安全。
	httpinfra.SweepStaleTempFiles()

	logger, err := infra.NewFileLogger(filepath.Join(configDir, wirexaConfigDir, "logs"))
	if err != nil {
		return fmt.Errorf("ロガーの初期化に失敗しました: %w", err)
	}
	adapters.SetupLogHandler(a.logHandler, logger)

	// MQTT / UDP の両サービスで共有する (ctx を保持するだけのステートレスな型)。
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
	a.netClient = httpinfra.NewNetClient()
	reqSvc := httpapp.NewHTTPRequestService(a.netClient, logger)
	adapters.SetupHTTPHandler(ctx, a.httpHandler, reqSvc, collSvc, collSvc, a.netClient)

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
	listenSvc := udpapp.NewUdpListenerService(udpSocket, emitter, logger)
	adapters.SetupUdpHandler(a.udpHandler, sendSvc, targetSvc, listenSvc)

	adapters.SetupOpenAPIHandler(ctx, a.openAPIHandler,
		filepath.Join(configDir, wirexaConfigDir, "openapi-recents.json"))

	a.restoreWindowState(ctx)

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

// restoreWindowState は保存済みのウィンドウサイズ・位置・最大化状態を復元する。
// 保存値が無い / 不正な場合は options.App の既定サイズのままにする。
func (a *App) restoreWindowState(ctx context.Context) {
	ws, ok := infra.LoadWindowState(a.windowStatePath)
	if !ok || ws.Width <= 0 || ws.Height <= 0 {
		return
	}
	a.winState = ws
	ws = a.clampWindowState(ctx, ws)
	runtime.WindowSetSize(ctx, ws.Width, ws.Height)
	runtime.WindowSetPosition(ctx, ws.X, ws.Y)
	if ws.Maximised {
		runtime.WindowMaximise(ctx)
	}
}

// clampWindowState は画面外にウィンドウが出ないようサイズ・位置を補正する。
// 注: Wails v2.12 の Screen はスクリーン原点を公開しないため、位置クランプは
// プライマリスクリーン基準の best-effort となる (マルチモニタの負座標はプライマリ側へ寄る)。
// 画面外化を保守的に防ぐことを優先する。
func (a *App) clampWindowState(ctx context.Context, ws infra.WindowState) infra.WindowState {
	screens, err := runtime.ScreenGetAll(ctx)
	if err != nil || len(screens) == 0 {
		return ws
	}
	sw, sh := 0, 0
	for _, s := range screens {
		if s.IsPrimary {
			sw, sh = s.Size.Width, s.Size.Height
			break
		}
	}
	if sw == 0 || sh == 0 {
		sw, sh = screens[0].Size.Width, screens[0].Size.Height
	}
	if sw <= 0 || sh <= 0 {
		return ws
	}

	if ws.Width > sw {
		ws.Width = sw
	}
	if ws.Height > sh {
		ws.Height = sh
	}
	if ws.Width < minWindowWidth {
		ws.Width = minWindowWidth
	}
	if ws.Height < minWindowHeight {
		ws.Height = minWindowHeight
	}

	maxX, maxY := sw-ws.Width, sh-ws.Height
	if ws.X < 0 {
		ws.X = 0
	}
	if ws.Y < 0 {
		ws.Y = 0
	}
	if ws.X > maxX {
		ws.X = maxX
	}
	if ws.Y > maxY {
		ws.Y = maxY
	}
	return ws
}

// saveWindowState は現在のウィンドウ状態を永続化する。ウィンドウが生存している
// beforeClose 内から呼ぶ。最大化中は通常時バウンズ (winState) を維持する。
func (a *App) saveWindowState() {
	if a.ctx == nil || a.windowStatePath == "" {
		return
	}
	maximised := runtime.WindowIsMaximised(a.ctx)
	if !maximised {
		w, h := runtime.WindowGetSize(a.ctx)
		x, y := runtime.WindowGetPosition(a.ctx)
		if w > 0 && h > 0 {
			a.winState.Width, a.winState.Height = w, h
			a.winState.X, a.winState.Y = x, y
		}
	}
	a.winState.Maximised = maximised
	_ = infra.SaveWindowState(a.windowStatePath, a.winState) //nolint:errcheck // best-effort persistence
}

// beforeClose はウィンドウを閉じようとしたときに呼ばれる。
// true を返すと閉じるのを阻止する。未保存文書の有無はフロントエンドしか
// 知らないため、"app:before-close" を通知して一旦阻止し、フロント側の判断
// (ConfirmQuit の呼び出し) を待つ。ConfirmQuit 経由で quitConfirmed が立てば
// 次の呼び出しでそのまま閉じる。
func (a *App) beforeClose(ctx context.Context) bool {
	// ウィンドウが生存しているこの時点で状態を保存する (冪等なので複数回呼ばれてよい)。
	a.saveWindowState()
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
	if a.netClient != nil {
		a.netClient.Cleanup()
	}
}
