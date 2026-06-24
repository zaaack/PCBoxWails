package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID          string
	Name        string
	Ws          *websocket.Conn
	ConnectedAt int64
}

type ClientManager struct {
	clients       map[string]*Client
	currentClient *Client
	mu            sync.RWMutex
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*Client),
	}
}

func (cm *ClientManager) Register(id, name string, ws *websocket.Conn) *Client {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	client := &Client{
		ID:          id,
		Name:        name,
		Ws:          ws,
		ConnectedAt: time.Now().UnixMilli(),
	}
	cm.clients[id] = client
	cm.currentClient = client
	log.Printf("Client registered: %s (%s)", name, id)
	return client
}

func (cm *ClientManager) RemoveByConnection(ws *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for id, client := range cm.clients {
		if client.Ws == ws {
			delete(cm.clients, id)
			if cm.currentClient != nil && cm.currentClient.ID == id {
				cm.currentClient = nil
			}
			return
		}
	}
}

func (cm *ClientManager) Get(id string) *Client {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.clients[id]
}

func (cm *ClientManager) GetAll() []ClientInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]ClientInfo, 0, len(cm.clients))
	for _, client := range cm.clients {
		result = append(result, ClientInfo{
			ID:          client.ID,
			Name:        client.Name,
			ConnectedAt: client.ConnectedAt,
		})
	}
	return result
}

func (cm *ClientManager) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.clients = make(map[string]*Client)
	cm.currentClient = nil
}

type WsMessage struct {
	Code      int         `json:"code"`
	Data      interface{} `json:"data"`
	TopicFlag *bool       `json:"topicFlag,omitempty"`
	TopicID   *string     `json:"topicId,omitempty"`
}

type WsServer struct {
	app           *App
	httpServer    *http.Server
	upgrader      websocket.Upgrader
	clientManager *ClientManager
	port          int
	running       bool
	mu            sync.RWMutex
}

func NewWsServer(app *App) *WsServer {
	return &WsServer{
		app:           app,
		clientManager: NewClientManager(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *WsServer) Start(port int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.stopLocked()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleUpgrade)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("WebSocket server error: %v", err)
		}
	}()

	s.port = port
	s.running = true
	log.Printf("WebSocket server started on port %d", port)
	return true
}

func (s *WsServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *WsServer) stopLocked() {
	if s.httpServer != nil {
		s.httpServer.Close()
		s.httpServer = nil
	}
	s.clientManager.Clear()
	s.running = false
}

func (s *WsServer) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"running": s.running,
		"port":    s.port,
	}
}

func (s *WsServer) SendMessage(clientID string, code int, data interface{}) bool {
	client := s.clientManager.Get(clientID)
	if client == nil {
		return false
	}

	dataMap, _ := data.(map[string]interface{})
	topicID := ""
	topicFlag := false

	if dataMap != nil {
		if tid, ok := dataMap["topicId"].(string); ok {
			topicID = tid
			topicFlag = tid != ""
		}
	}

	var sentData interface{} = data
	if dataMap != nil {
		cleaned := make(map[string]interface{})
		for k, v := range dataMap {
			if k != "topicId" {
				cleaned[k] = v
			}
		}
		if len(cleaned) == 0 {
			sentData = nil
		} else {
			sentData = cleaned
		}
	}

	msg := map[string]interface{}{
		"code":      code,
		"data":      sentData,
		"topicFlag": topicFlag,
		"topicId":   topicID,
	}

	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return false
	}

	if client.Ws.WriteMessage(websocket.TextMessage, jsonBytes) != nil {
		return false
	}

	log.Printf("Sent message: %s", string(jsonBytes))
	return true
}

func (s *WsServer) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	log.Println("New WebSocket connection")
	s.handleConnection(conn)
}

func (s *WsServer) handleConnection(ws *websocket.Conn) {
	defer func() {
		s.clientManager.RemoveByConnection(ws)
		s.app.emitEvent("client-disconnected", nil)
		ws.Close()
	}()

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
		}

		var msg WsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			ws.Close()
			return
		}

		s.dispatch(msg, ws)
	}
}

func (s *WsServer) dispatch(msg WsMessage, ws *websocket.Conn) {
	log.Printf("Received message: code=%d", msg.Code)

	switch msg.Code {
	case 100: // REGISTER
		s.handleRegister(msg, ws)
	case 202, 204, 206, 208, 210, 212, 214, 218, 220, 222, 224, 226, 228, 230: // *_RESULT
		s.handleTopicResponse(msg)
	default:
		log.Printf("Unknown message code: %d", msg.Code)
	}
}

func (s *WsServer) handleRegister(msg WsMessage, ws *websocket.Conn) {
	dataMap, ok := msg.Data.(map[string]interface{})
	if !ok || dataMap == nil {
		ws.Close()
		return
	}

	clientID, _ := dataMap["clientId"].(string)
	if clientID == "" {
		ws.Close()
		return
	}

	clientName, _ := dataMap["clientName"].(string)
	if clientName == "" {
		clientName = "Unknown"
	}

	client := s.clientManager.Register(clientID, clientName, ws)

	s.app.emitEvent("client-connected", map[string]interface{}{
		"id":   client.ID,
		"name": client.Name,
	})

	log.Printf("Client registered: %s", client.Name)
}

func (s *WsServer) handleTopicResponse(msg WsMessage) {
	if msg.TopicID != nil {
		s.app.emitEvent("ws-response", map[string]interface{}{
			"topicId": *msg.TopicID,
			"code":    msg.Code,
			"data":    msg.Data,
		})
	}
}
