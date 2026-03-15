// Package main is the entry point for the Wirexa desktop application.
package main

import (
	"context"

	httpservice "github.com/f0reth/Wirexa/internal/http"
	"github.com/f0reth/Wirexa/internal/mqtt"
)

type App struct {
	ctx         context.Context
	mqttService *mqtt.MqttService
	httpService *httpservice.HttpService
}

func NewApp() *App {
	return &App{
		mqttService: mqtt.NewMqttService(),
		httpService: httpservice.NewHTTPService(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.mqttService.SetContext(ctx)
	a.httpService.SetContext(ctx)
}

func (a *App) shutdown(_ context.Context) {
	a.mqttService.Shutdown()
	a.httpService.Shutdown()
}
