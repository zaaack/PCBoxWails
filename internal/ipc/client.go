package ipc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type IPCClient struct {
	serverURL  string
	wsURL      string
	httpClient *http.Client
	wsConn     *websocket.Conn
	handlers   map[string][]func(data interface{})
	handlersMu sync.RWMutex
	done       chan struct{}
}

func NewIPCClient(port int) *IPCClient {
	return &IPCClient{
		serverURL:  fmt.Sprintf("http://127.0.0.1:%d", port),
		wsURL:      fmt.Sprintf("ws://127.0.0.1:%d/ws", port),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		handlers:   make(map[string][]func(data interface{})),
		done:       make(chan struct{}),
	}
}

func (c *IPCClient) Connect() error {
	var err error
	for i := 0; i < 50; i++ {
		resp, e := http.Get(c.serverURL + "/health")
		if e == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				go c.connectWebSocket()
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
		err = e
	}
	return fmt.Errorf("failed to connect to server after retries: %w", err)
}

func (c *IPCClient) connectWebSocket() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.Dial(c.wsURL, nil)
		if err != nil {
			log.Printf("[IPC Client] WebSocket failed: %v, retrying...", err)
			time.Sleep(time.Second)
			continue
		}

		c.wsConn = conn
		log.Println("[IPC Client] WebSocket connected")

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[IPC Client] WebSocket read error: %v", err)
				break
			}

			var event Event
			if err := json.Unmarshal(message, &event); err != nil {
				continue
			}

			c.handlersMu.RLock()
			handlers := c.handlers[event.Name]
			c.handlersMu.RUnlock()

			for _, h := range handlers {
				go h(event.Data)
			}
		}

		conn.Close()
		c.wsConn = nil
		time.Sleep(time.Second)
	}
}

func (c *IPCClient) OnEvent(name string, handler func(data interface{})) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[name] = append(c.handlers[name], handler)
}

func (c *IPCClient) Call(method string, args interface{}) (interface{}, error) {
	reqBody, err := json.Marshal(map[string]interface{}{"args": args})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(c.serverURL+"/api/"+method, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("call %s failed: %w", method, err)
	}
	defer resp.Body.Close()

	var result methodResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, fmt.Errorf("%s", result.Error)
	}

	return result.Result, nil
}

func (c *IPCClient) Close() {
	close(c.done)
	if c.wsConn != nil {
		c.wsConn.Close()
	}
}
