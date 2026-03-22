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
	httpinfra "github.com/f0reth/Wirexa/internal/infrastructure/http"
	mqttinfra "github.com/f0reth/Wirexa/internal/infrastructure/mqtt"
)

type App struct {
	mqttHandler *adapters.MqttHandler
	httpHandler *adapters.HttpHandler
}

func NewApp() *App {
	return &App{
		mqttHandler: &adapters.MqttHandler{},
		httpHandler: &adapters.HttpHandler{},
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
	if err := collSvc.Initialize(); err != nil {
		log.Fatalf("startup: failed to initialize collections: %v", err)
	}
	reqSvc := httpapp.NewHTTPRequestService()
	adapters.SetupHTTPHandler(a.httpHandler, reqSvc, collSvc)
}

func (a *App) shutdown(_ context.Context) {
	a.mqttHandler.Shutdown()
}
