package ipc

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type MethodHandler func(args json.RawMessage) (interface{}, error)

type Event struct {
	Name string      `json:"name"`
	Data interface{} `json:"data"`
}

type IPCServer struct {
	methods   map[string]MethodHandler
	upgrader  websocket.Upgrader
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
	port      int
}

func NewIPCServer() *IPCServer {
	return &IPCServer{
		methods:  make(map[string]MethodHandler),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		clients:  make(map[*websocket.Conn]bool),
	}
}

func (s *IPCServer) RegisterMethod(name string, handler MethodHandler) {
	s.methods[name] = handler
}

func (s *IPCServer) EmitEvent(name string, data interface{}) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	msg, _ := json.Marshal(Event{Name: name, Data: data})
	for conn := range s.clients {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("[IPC] Failed to send event: %v", err)
		}
	}
}

func (s *IPCServer) Start(port int) error {
	s.port = port

	mux := http.NewServeMux()
	mux.HandleFunc("/api/", s.handleMethod)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("[IPC] Server starting on %s", addr)
	return http.ListenAndServe(addr, mux)
}

type methodRequest struct {
	Args json.RawMessage `json:"args"`
}

type methodResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func (s *IPCServer) handleMethod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	method := r.URL.Path[len("/api/"):]
	handler, ok := s.methods[method]
	if !ok {
		http.Error(w, fmt.Sprintf("Unknown method: %s", method), http.StatusNotFound)
		return
	}

	var req methodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, methodResponse{Error: "Invalid request body"})
		return
	}

	result, err := handler(req.Args)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, methodResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, methodResponse{Result: result})
}

func (s *IPCServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[IPC] WebSocket upgrade error: %v", err)
		return
	}

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	log.Println("[IPC] New client connected")

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
