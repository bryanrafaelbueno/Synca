package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	syncengine "github.com/synca/daemon/internal/sync"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" ||
			origin == "tauri://localhost" ||
			origin == "http://tauri.localhost" ||
			origin == "https://tauri.localhost" ||
			origin == "http://localhost:1420" ||
			origin == "http://localhost:5173"
	},
}

// WebSocketServer streams sync status to connected clients.
type WebSocketServer struct {
	engine  *syncengine.Engine
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func NewWebSocketServer(engine *syncengine.Engine) *WebSocketServer {
	s := &WebSocketServer{
		engine:  engine,
		clients: make(map[*websocket.Conn]struct{}),
	}
	go s.broadcastLoop()
	return s
}

// Start begins listening on addr
func (s *WebSocketServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	log.Info().Str("addr", addr).Msg("WebSocket server listening")
	return http.ListenAndServe(addr, mux)
}

func (s *WebSocketServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("WS upgrade failed")
		return
	}
	defer conn.Close()

	s.mu.Lock()
	s.clients[conn] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
	}()

	// initial snapshot
	snap := s.engine.Snapshot()
	if data, err := json.Marshal(snap); err == nil {
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		s.handleCommand(conn, msg)
	}
}

func (s *WebSocketServer) writeWSError(conn *websocket.Conn, text string) {
	payload, _ := json.Marshal(map[string]string{"error": text})
	_ = conn.WriteMessage(websocket.TextMessage, payload)
}

func (s *WebSocketServer) handleCommand(conn *websocket.Conn, msg []byte) {
	var in struct {
		Action string `json:"action"`
		Path   string `json:"path"`
	}

	if err := json.Unmarshal(msg, &in); err != nil {
		return
	}

	switch in.Action {
	case "get_status":
		snap := s.engine.Snapshot()
		if data, err := json.Marshal(snap); err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}
	case "add_watch":
		if strings.TrimSpace(in.Path) == "" {
			s.writeWSError(conn, "folder path is missing")
			return
		}
		if err := s.engine.AddWatchRoot(context.Background(), in.Path); err != nil {
			s.writeWSError(conn, err.Error())
			return
		}
		snap := s.engine.Snapshot()
		if data, err := json.Marshal(snap); err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}
	case "restart_daemon":
		// Re-exec same binary as `… daemon` so the process comes back without relying on systemd.
		go func() {
			time.Sleep(100 * time.Millisecond)
			log.Info().Msg("Restart requested via WebSocket — re-exec as daemon")
			if err := restartProcessAsDaemon(); err != nil {
				log.Warn().Err(err).Msg("re-exec failed, exiting (supervisor may restart)")
				os.Exit(0)
			}
		}()
	}
}

func (s *WebSocketServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	snap := s.engine.Snapshot()
	_ = json.NewEncoder(w).Encode(snap)
}

func (s *WebSocketServer) broadcastLoop() {
	for snap := range s.engine.Updates {
		data, err := json.Marshal(snap)
		if err != nil {
			continue
		}

		s.mu.Lock()
		for conn := range s.clients {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				conn.Close()
				delete(s.clients, conn)
			}
		}
		s.mu.Unlock()
	}
}

// PrintStatus connects to the daemon and prints status
func PrintStatus() error {
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:7373/ws", nil)
	if err != nil {
		return fmt.Errorf("daemon not running (start with: synca daemon)")
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	_, msg, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	var snap syncengine.StatusSnapshot
	if err := json.Unmarshal(msg, &snap); err != nil {
		return err
	}

	fmt.Printf("Synca status — %s\n", snap.LastUpdated.Format("15:04:05"))
	fmt.Printf("Files: %d synced / %d total\n", snap.SyncedFiles, snap.TotalFiles)
	fmt.Printf("Data:  %.1f MB\n", float64(snap.TotalBytes)/1e6)

	for _, f := range snap.Files {
		fmt.Printf("  [%s] %s\n", f.Status, f.LocalPath)
	}

	return nil
}

// IsDaemonRunning checks if daemon is up
func IsDaemonRunning() bool {
	conn, err := net.DialTimeout("tcp", "localhost:7373", time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
