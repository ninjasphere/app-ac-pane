package main

import (
	"fmt"

	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/go-ninja/config"
	"github.com/ninjasphere/go-ninja/logger"
	"github.com/ninjasphere/go-ninja/support"
	"github.com/ninjasphere/sphere-go-led-controller/remote"
)

var log = logger.GetLogger("ac-pane")

var host = config.String("localhost", "led.host")
var port = config.Int(3115, "led.remote.port")

func main() {

	app := &App{}
	err := app.Init(info)
	if err != nil {
		app.Log.Fatalf("failed to initialize app: %v", err)
	}

	err = app.Export(app)
	if err != nil {
		app.Log.Fatalf("failed to export app: %v", err)
	}

	support.WaitUntilSignal()
}

var info = ninja.LoadModuleInfo("./package.json")

type Config struct {
}

type App struct {
	support.AppSupport
	led *remote.Matrix
}

func (a *App) Start(cfg *Config) error {

	// Create our pane. Must implement (github.com/ninjasphere/go-ninja/remote).pane
	pane := NewACPane(a.Conn)

	// Export our pane over this interface
	a.led = remote.NewTCPMatrix(pane, fmt.Sprintf("%s:%d", host, port))

	return nil
}

// Stop the security light app.
func (a *App) Stop() error {
	a.led.Close()
	a.led = nil
	return nil
}
