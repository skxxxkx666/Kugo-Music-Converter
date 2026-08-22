package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

var (
	version    = "v0.6.1-dev"
	buildDate  = "unknown"
	commitHash = "unknown"
)

//go:embed all:frontend/src
var embeddedFrontend embed.FS

const windowTitle = "Kugo Music Converter"

func main() {
	if handled, exitCode := runReleaseSelfTest(os.Args[1:]); handled {
		os.Exit(exitCode)
	}

	releaseInstance, ok := enforceSingleInstance(windowTitle)
	if !ok {
		// 已有实例在运行，enforceSingleInstance 已把它带到前台
		return
	}
	defer releaseInstance()

	webviewBrowserPath, err := prepareWebView2Runtime()
	if err != nil {
		showStartupError("内置 WebView2 运行时准备失败", err.Error())
		return
	}

	assets, err := fs.Sub(embeddedFrontend, "frontend/src")
	if err != nil {
		panic(fmt.Errorf("load desktop frontend: %w", err))
	}

	app := NewApp(version)
	err = wails.Run(&options.App{
		Title:     windowTitle,
		Width:     1120,
		Height:    760,
		MinWidth:  920,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.assetHandler(),
		},
		BackgroundColour: options.NewRGB(246, 248, 251),
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		Bind:             []interface{}{app},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		Windows: &windows.Options{
			Theme:                windows.SystemDefault,
			IsZoomControlEnabled: false,
			DisablePinchZoom:     true,
			WebviewBrowserPath:   webviewBrowserPath,
		},
	})
	if err != nil {
		panic(fmt.Errorf("start desktop app: %w", err))
	}
}
