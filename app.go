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
	httpinfra "github.com/f0reth/Wirexa/internal/infrastructure/http"
	mqttinfra "github.com/f0reth/Wirexa/internal/infrastructure/mqtt"
	udpinfra "github.com/f0reth/Wirexa/internal/infrastructure/udp"
)

type App struct {
	mqttHandler *adapters.MqttHandler
	httpHandler *adapters.HttpHandler
	udpHandler  *adapters.UdpHandler
}

func NewApp() *App {
	return &App{
		mqttHandler: &adapters.MqttHandler{},
		httpHandler: &adapters.HttpHandler{},
		udpHandler:  &adapters.UdpHandler{},
	}
}

func (a *App) startup(ctx context.Context) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("startup: failed to get user config dir: %v", err)
	}

	emitter := mqttinfra.NewWailsEmitter(ctx)
	clientFactory := mqttinfra.NewPahoClientFactory()
	mqttSvc := mqttapp.NewMqttService(emitter, clientFactory)

	profileRepo, err := mqttinfra.NewJSONProfileRepository(filepath.Join(configDir, "Wirexa", "mqtt-profiles"))
	if err != nil {
		log.Fatalf("startup: failed to create profile repository: %v", err)
	}
	profileSvc, err := mqttapp.NewProfileService(profileRepo)
	if err != nil {
		log.Fatalf("startup: failed to create profile service: %v", err)
	}
	adapters.SetupMqttHandler(a.mqttHandler, mqttSvc, profileSvc)

	collRepo, err := httpinfra.NewJSONFileRepository(filepath.Join(configDir, "Wirexa", "collections"))
	if err != nil {
		log.Fatalf("startup: failed to create collection repository: %v", err)
	}
	collSvc := httpapp.NewCollectionService(collRepo)
	if err = collSvc.Initialize(); err != nil {
		log.Fatalf("startup: failed to initialize collections: %v", err)
	}
	reqSvc := httpapp.NewHTTPRequestService()
	adapters.SetupHTTPHandler(a.httpHandler, reqSvc, collSvc)

	targetRepo, err := udpinfra.NewJSONTargetRepository(filepath.Join(configDir, "Wirexa", "udp-targets"))
	if err != nil {
		log.Fatalf("startup: failed to create target repository: %v", err)
	}
	targetSvc, err := udpapp.NewTargetService(targetRepo)
	if err != nil {
		log.Fatalf("startup: failed to create target service: %v", err)
	}
	udpSocket := udpinfra.NewNetSocket()
	sendSvc := udpapp.NewUdpSendService(udpSocket)
	udpEmitter := udpinfra.NewWailsEmitter(ctx)
	listenSvc := udpapp.NewUdpListenerService(udpSocket, udpEmitter)
	adapters.SetupUdpHandler(a.udpHandler, sendSvc, targetSvc, listenSvc)
}

func (a *App) shutdown(_ context.Context) {
	a.mqttHandler.Shutdown()
	a.udpHandler.Shutdown()
}
