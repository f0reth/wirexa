// Package main is the entry point for the Wirexa desktop application.
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/f0reth/Wirexa/internal/adapters"
	httpapp "github.com/f0reth/Wirexa/internal/application/http"
	mqttapp "github.com/f0reth/Wirexa/internal/application/mqtt"
	udpapp "github.com/f0reth/Wirexa/internal/application/udp"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
	httpinfra "github.com/f0reth/Wirexa/internal/infrastructure/http"
	mqttinfra "github.com/f0reth/Wirexa/internal/infrastructure/mqtt"
	udpinfra "github.com/f0reth/Wirexa/internal/infrastructure/udp"
)

const wirexaConfigDir = "Wirexa"

type App struct {
	mqttHandler    *adapters.MqttHandler
	httpHandler    *adapters.HttpHandler
	udpHandler     *adapters.UdpHandler
	logHandler     *adapters.LogHandler
	openAPIHandler *adapters.OpenAPIHandler
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
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("startup: failed to get user config dir: %v", err)
	}

	logger, err := infra.NewFileLogger(filepath.Join(configDir, wirexaConfigDir, "logs"))
	if err != nil {
		log.Fatalf("startup: failed to create logger: %v", err)
	}
	adapters.SetupLogHandler(a.logHandler, logger)

	emitter := infra.NewWailsEmitter(ctx)
	clientFactory := mqttinfra.NewPahoClientFactory(mqttinfra.MqttClientConfig{})
	mqttSvc := mqttapp.NewMqttService(emitter, clientFactory, logger)

	profileRepo, err := mqttinfra.NewJSONProfileRepository(filepath.Join(configDir, wirexaConfigDir, "mqtt-profiles"))
	if err != nil {
		log.Fatalf("startup: failed to create profile repository: %v", err)
	}
	profileSvc, err := mqttapp.NewProfileService(profileRepo)
	if err != nil {
		log.Fatalf("startup: failed to create profile service: %v", err)
	}
	adapters.SetupMqttHandler(a.mqttHandler, mqttSvc, profileSvc)

	collRepo, err := httpinfra.NewJSONFileRepository(filepath.Join(configDir, wirexaConfigDir, "collections"))
	if err != nil {
		log.Fatalf("startup: failed to create collection repository: %v", err)
	}
	collSvc, err := httpapp.NewCollectionService(collRepo)
	if err != nil {
		log.Fatalf("startup: failed to initialize collections: %v", err)
	}
	netClient := httpinfra.NewNetClient()
	reqSvc := httpapp.NewHTTPRequestService(netClient, logger)
	adapters.SetupHTTPHandler(ctx, a.httpHandler, reqSvc, collSvc, collSvc)

	targetRepo, err := udpinfra.NewJSONTargetRepository(filepath.Join(configDir, wirexaConfigDir, "udp-targets"))
	if err != nil {
		log.Fatalf("startup: failed to create target repository: %v", err)
	}
	targetSvc, err := udpapp.NewTargetService(targetRepo)
	if err != nil {
		log.Fatalf("startup: failed to create target service: %v", err)
	}
	udpSocket := udpinfra.NewNetSocket()
	sendSvc := udpapp.NewUdpSendService(udpSocket, logger)
	udpEmitter := infra.NewWailsEmitter(ctx)
	listenSvc := udpapp.NewUdpListenerService(udpSocket, udpEmitter, logger)
	adapters.SetupUdpHandler(a.udpHandler, sendSvc, targetSvc, listenSvc)

	adapters.SetupOpenAPIHandler(ctx, a.openAPIHandler)
}

func (a *App) shutdown(_ context.Context) {
	a.mqttHandler.Shutdown()
	a.udpHandler.Shutdown()
}
