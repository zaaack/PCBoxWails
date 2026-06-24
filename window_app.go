package main

import (
	"context"
	"log"

	"PcBoxWails/internal/ipc"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type WindowApp struct {
	ctx       context.Context
	ipcClient *ipc.IPCClient
}

func NewWindowApp(ipcPort int) *WindowApp {
	return &WindowApp{
		ipcClient: ipc.NewIPCClient(ipcPort),
	}
}

func (a *WindowApp) startup(ctx context.Context) {
	a.ctx = ctx
	a.bridgeEvents()
}

func (a *WindowApp) bridgeEvents() {
	a.ipcClient.OnEvent("client-connected", func(data interface{}) {
		runtime.EventsEmit(a.ctx, "client-connected", data)
	})
	a.ipcClient.OnEvent("client-disconnected", func(data interface{}) {
		runtime.EventsEmit(a.ctx, "client-disconnected")
	})
	a.ipcClient.OnEvent("ws-response", func(data interface{}) {
		runtime.EventsEmit(a.ctx, "ws-response", data)
	})
}

func (a *WindowApp) StartWsServer(port int) bool {
	result, err := a.ipcClient.Call("StartWsServer", port)
	if err != nil {
		log.Printf("[Window] StartWsServer error: %v", err)
		return false
	}
	return toBool(result)
}

func (a *WindowApp) StopWsServer() bool {
	result, err := a.ipcClient.Call("StopWsServer", nil)
	if err != nil {
		log.Printf("[Window] StopWsServer error: %v", err)
		return false
	}
	return toBool(result)
}

func (a *WindowApp) GetWsServerStatus() map[string]interface{} {
	log.Println("[Window] GetWsServerStatus called")
	result, err := a.ipcClient.Call("GetWsServerStatus", nil)
	if err != nil {
		log.Printf("[Window] GetWsServerStatus error: %v", err)
		return map[string]interface{}{"running": false, "port": 0}
	}
	log.Printf("[Window] GetWsServerStatus result: %v", result)
	return toMap(result)
}

func (a *WindowApp) GetLocalIp() string {
	result, err := a.ipcClient.Call("GetLocalIp", nil)
	if err != nil {
		log.Printf("[Window] GetLocalIp error: %v", err)
		return "127.0.0.1"
	}
	return toString(result)
}

func (a *WindowApp) GetClients() []map[string]interface{} {
	result, err := a.ipcClient.Call("GetClients", nil)
	if err != nil {
		log.Printf("[Window] GetClients error: %v", err)
		return []map[string]interface{}{}
	}
	return toSlice(result)
}

func (a *WindowApp) SendMessage(clientId string, code int, data interface{}) bool {
	result, err := a.ipcClient.Call("SendMessage", map[string]interface{}{
		"clientId": clientId,
		"code":     code,
		"data":     data,
	})
	if err != nil {
		log.Printf("[Window] SendMessage error: %v", err)
		return false
	}
	return toBool(result)
}

func (a *WindowApp) CreateProxySession(url string, headers map[string]string) string {
	result, err := a.ipcClient.Call("CreateProxySession", map[string]interface{}{
		"url":     url,
		"headers": headers,
	})
	if err != nil {
		log.Printf("[Window] CreateProxySession error: %v", err)
		return ""
	}
	return toString(result)
}

func toBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func toSlice(v interface{}) []map[string]interface{} {
	if arr, ok := v.([]interface{}); ok {
		result := make([]map[string]interface{}, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result
	}
	return []map[string]interface{}{}
}
