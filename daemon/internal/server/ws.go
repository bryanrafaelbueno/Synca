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

	"github.com/synca/daemon/internal/config"
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

// SafeConn wraps a gorilla websocket connection with a mutex to ensure thread-safe writes.
type SafeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (s *SafeConn) WriteMessage(messageType int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(messageType, data)
}

func (s *SafeConn) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.Close()
}

func (s *SafeConn) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// WebSocketServer streams sync status to connected clients.
type WebSocketServer struct {
	engine  *syncengine.Engine
	mu      sync.Mutex
	clients map[*SafeConn]struct{}
	cancel  context.CancelFunc
}

func NewWebSocketServer(engine *syncengine.Engine, cancel context.CancelFunc) *WebSocketServer {
	s := &WebSocketServer{
		engine:  engine,
		clients: make(map[*SafeConn]struct{}),
		cancel:  cancel,
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
	mux.HandleFunc("/account", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		client := s.engine.DriveClient()
		if client == nil {
			http.Error(w, `{"error":"Drive client not initialized"}`, http.StatusInternalServerError)
			return
		}

		about, err := client.GetAbout(r.Context())
		if err != nil {
			s.engine.RecordNetworkError(err)
			log.Error().Err(err).Msg("Failed to fetch Google Drive profile")
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		s.engine.ClearNetworkError()

		res := map[string]interface{}{
			"email":        about.User.EmailAddress,
			"display_name": about.User.DisplayName,
			"photo_url":    about.User.PhotoLink,
			"used_bytes":   about.StorageQuota.Usage,
			"total_bytes":  about.StorageQuota.Limit,
		}
		_ = json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/quit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			if !upgrader.CheckOrigin(r) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			log.Info().Msg("Received quit signal via HTTP, shutting down gracefully...")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			go func() {
				time.Sleep(100 * time.Millisecond)
				if s.cancel != nil {
					s.cancel()
				} else {
					os.Exit(0)
				}
			}()
		}
	})

	log.Info().Str("addr", addr).Msg("WebSocket server listening")

	var listener net.Listener
	var err error
	for i := 0; i < 10; i++ {
		listener, err = net.Listen("tcp", addr)
		if err == nil {
			break
		}
		log.Warn().Err(err).Msgf("Failed to bind to %s, retrying in 500ms... (%d/10)", addr, i+1)
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("failed to bind port after retries: %w", err)
	}

	return http.Serve(listener, mux)
}

func (s *WebSocketServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("WS upgrade failed")
		return
	}
	sconn := &SafeConn{conn: conn}
	defer sconn.Close()

	s.mu.Lock()
	s.clients[sconn] = struct{}{}
	s.mu.Unlock()
	log.Info().Str("remote", sconn.RemoteAddr().String()).Msg("WebSocket client connected")

	defer func() {
		s.mu.Lock()
		delete(s.clients, sconn)
		s.mu.Unlock()
		log.Info().Str("remote", sconn.RemoteAddr().String()).Msg("WebSocket client disconnected")
	}()

	// initial snapshot
	snap := s.engine.Snapshot()
	if data, err := json.Marshal(snap); err == nil {
		_ = sconn.WriteMessage(websocket.TextMessage, data)
		log.Debug().Int("files", len(snap.Files)).Msg("Sent initial snapshot to client")
	}

	for {
		_, msg, err := sconn.conn.ReadMessage()
		if err != nil {
			return
		}
		s.handleCommand(sconn, msg)
	}
}

func (s *WebSocketServer) writeWSError(sconn *SafeConn, text string) {
	payload, _ := json.Marshal(map[string]string{"error": text})
	_ = sconn.WriteMessage(websocket.TextMessage, payload)
}

func (s *WebSocketServer) handleCommand(sconn *SafeConn, msg []byte) {
	var in struct {
		Action         string               `json:"action"`
		Path           string               `json:"path"`
		Mode           string               `json:"mode"`
		IgnoredFolders []string             `json:"ignored_folders"`
		Proxy          config.ProxySettings `json:"proxy"`
	}

	if err := json.Unmarshal(msg, &in); err != nil {
		return
	}

	switch in.Action {
	case "get_status":
		snap := s.engine.Snapshot()
		if data, err := json.Marshal(snap); err == nil {
			_ = sconn.WriteMessage(websocket.TextMessage, data)
		}
	case "add_watch":
		if strings.TrimSpace(in.Path) == "" {
			s.writeWSError(sconn, "folder path is missing")
			return
		}
		mode := config.ParseSyncMode(in.Mode)
		if err := s.engine.AddWatchRootWithMode(context.Background(), in.Path, mode); err != nil {
			s.writeWSError(sconn, err.Error())
			return
		}
		snap := s.engine.Snapshot()
		if data, err := json.Marshal(snap); err == nil {
			_ = sconn.WriteMessage(websocket.TextMessage, data)
		}
	case "update_watch":
		if strings.TrimSpace(in.Path) == "" {
			s.writeWSError(sconn, "folder path is missing")
			return
		}
		mode := config.ParseSyncMode(in.Mode)
		if err := s.engine.UpdateWatchMode(in.Path, mode); err != nil {
			s.writeWSError(sconn, err.Error())
			return
		}
		snap := s.engine.Snapshot()
		if data, err := json.Marshal(snap); err == nil {
			_ = sconn.WriteMessage(websocket.TextMessage, data)
		}
	case "remove_watch":
		if in.Path == "" {
			s.writeWSError(sconn, "folder path is missing")
			return
		}
		go func() {
			if err := s.engine.RemoveWatchRoot(context.Background(), in.Path); err != nil {
				s.writeWSError(sconn, err.Error())
			}
		}()
	case "set_ignored_folders":
		if err := s.engine.SetIgnoredFolders(in.IgnoredFolders); err != nil {
			s.writeWSError(sconn, err.Error())
			return
		}
		snap := s.engine.Snapshot()
		if data, err := json.Marshal(snap); err == nil {
			_ = sconn.WriteMessage(websocket.TextMessage, data)
		}
	case "set_proxy":
		if err := s.engine.SetProxySettings(context.Background(), in.Proxy); err != nil {
			s.writeWSError(sconn, err.Error())
			return
		}
		snap := s.engine.Snapshot()
		if data, err := json.Marshal(snap); err == nil {
			_ = sconn.WriteMessage(websocket.TextMessage, data)
		}
	case "restart_daemon":
		// Re-exec same binary as `… daemon` so the process comes back without relying on systemd.
		go func() {
			time.Sleep(100 * time.Millisecond)
			log.Info().Msg("Restart requested via WebSocket — re-exec as daemon")
			if err := restartProcessAsDaemon(); err != nil {
				log.Warn().Err(err).Msg("re-exec failed, exiting gracefully")
			}
			if s.cancel != nil {
				s.cancel()
			} else {
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
		for sconn := range s.clients {
			if err := sconn.WriteMessage(websocket.TextMessage, data); err != nil {
				sconn.Close()
				delete(s.clients, sconn)
			}
		}
		s.mu.Unlock()
	}
}

// PrintStatus connects to the daemon and prints status
func PrintStatus() error {
	addr := "localhost:7373"
	if cfg, err := config.Load(); err == nil {
		addr = cfg.WSAddr
	}

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/ws", nil)
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
	addr := "localhost:7373"
	if cfg, err := config.Load(); err == nil {
		addr = cfg.WSAddr
	}

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
