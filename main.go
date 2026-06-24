package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"PcBoxWails/internal/ipc"
	"PcBoxWails/internal/tray"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var iconPNG []byte

func main() {
	mode := flag.String("mode", "server", "Run mode: standalone, server, window")
	ipcPort := flag.Int("ipc-port", 9899, "IPC server port")
	flag.Parse()

	if envMode := os.Getenv("PCBOX_MODE"); envMode != "" {
		*mode = envMode
	}

	switch *mode {
	case "server":
		runServer(*ipcPort)
	case "window":
		runWindow(*ipcPort)
	default:
		runStandalone()
	}
}

func isWailsDev() bool {
	exe, err := os.Executable()
	if err != nil {
		return true
	}
	lower := strings.ToLower(exe)
	return strings.Contains(lower, "wails") ||
		strings.Contains(lower, "__debug") ||
		strings.Contains(lower, "\\temp\\") ||
		strings.Contains(lower, "/tmp/")
}

func shouldOpenDevTools() bool {
	if isWailsDev() {
		return true
	}
	return os.Getenv("PCBOX_DEVTOOLS") == "1"
}

func runStandalone() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "PCBox",
		Width:     800,
		Height:    500,
		MinWidth:  600,
		MinHeight: 400,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 15, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.pcbox.app",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				log.Printf("Second instance launched: %v", secondInstanceData.Args)
			},
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: shouldOpenDevTools(),
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func runServer(ipcPort int) {
// 	logFile, err := os.Create("pcbox-server.log")
// 	if err == nil {
// 		log.SetOutput(logFile)
// 		defer logFile.Close()
// 	}

	srv := &ServerApp{}
	srv.startup()
	defer srv.shutdown()

	ipcSrv := ipc.NewIPCServer()
	srv.ipcServer = ipcSrv
	registerIPCMethods(srv, ipcSrv)

	go func() {
		if err := ipcSrv.Start(ipcPort); err != nil {
			log.Fatalf("[IPC] Server failed: %v", err)
		}
	}()

	t := tray.New()
	if len(iconPNG) > 0 {
		t.SetIcon(iconPNG)
	}
	t.SetTooltip("PCBox Server")

	menu := tray.NewMenu()
	menu.Add("显示窗口", func() { showWindow(srv) })
	menu.AddSeparator()
	menu.Add("退出", func() {
		t.Remove()
		if srv.windowCmd != nil && srv.windowCmd.Process != nil {
			srv.windowCmd.Process.Kill()
		}
		os.Exit(0)
	})
	t.SetMenu(menu)
	t.OnDoubleClick(func() { showWindow(srv) })

	t.Show()
	
	if envBuild := os.Getenv("PCBOX_BUILD"); envBuild != "" {
    		runWindow(ipcPort)
    		return
	}
	
	go showWindow(srv)
	if err := t.Run(); err != nil {
		log.Fatalf("[Tray] Run error: %v", err)
	}
}

func runWindow(ipcPort int) {
	wapp := NewWindowApp(ipcPort)

	if err := wapp.ipcClient.Connect(); err != nil {
		log.Fatalf("[Window] Failed to connect to server: %v", err)
	}
	defer wapp.ipcClient.Close()

	err := wails.Run(&options.App{
		Title:     "PCBox",
		Width:     800,
		Height:    500,
		MinWidth:  600,
		MinHeight: 400,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 15, A: 1},
		OnStartup:        wapp.startup,
		Bind: []interface{}{
			wapp,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.pcbox.window",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				log.Printf("Window second instance: %v", secondInstanceData.Args)
			},
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: shouldOpenDevTools(),
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func showWindow(srv *ServerApp) {
	if srv.windowCmd != nil && srv.windowCmd.Process != nil {
		log.Println("[Server] Window already running")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		log.Printf("[Server] Failed to get exe path: %v", err)
		return
	}

	srv.windowCmd = exec.Command(exe, "--mode=window", "--ipc-port=9899")
	srv.windowCmd.Stdout = os.Stdout
	srv.windowCmd.Stderr = os.Stderr

	if err := srv.windowCmd.Start(); err != nil {
		log.Printf("[Server] Failed to start window: %v", err)
		srv.windowCmd = nil
		return
	}

	go func() {
		srv.windowCmd.Wait()
		srv.windowCmd = nil
		log.Println("[Server] Window process exited")
	}()
}

func init() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Exit(0)
	}()
}
