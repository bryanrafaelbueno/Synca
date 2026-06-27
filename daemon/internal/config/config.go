package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ProxyMode string
type ProxyType string

const (
	ProxyModeNone   ProxyMode = "none"
	ProxyModeSystem ProxyMode = "system"
	ProxyModeManual ProxyMode = "manual"

	ProxyTypeHTTP  ProxyType = "http"
	ProxyTypeSOCKS ProxyType = "socks"
)

type ProxySettings struct {
	Mode               ProxyMode `json:"mode"`
	Type               ProxyType `json:"type"`
	Host               string    `json:"host"`
	Port               string    `json:"port"`
	Username           string    `json:"username"`
	Password           string    `json:"password"`
	InsecureSkipVerify bool      `json:"insecure_skip_verify,omitempty"`
}

type Config struct {
	WSAddr         string              `json:"ws_addr"`
	WatchPaths     []string            `json:"watch_paths"`
	WatchPathModes map[string]SyncMode `json:"watch_path_modes,omitempty"`
	IgnoredFolders []string            `json:"ignored_folders,omitempty"`
	Proxy          ProxySettings       `json:"proxy"`
	TokenFile      string              `json:"token_file"`
	CredFile       string              `json:"cred_file"`
	ConflictDir    string              `json:"conflict_dir"`
	LogLevel       string              `json:"log_level"`
	// Internal (not persisted)
	configPath string `json:"-"`
}

func configDir() (string, error) {
	// Use os.UserConfigDir() for cross-platform support:
	//   Linux:   $HOME/.config
	//   Windows: %APPDATA%
	//   macOS:   $HOME/Library/Application Support
	dir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to $HOME/.config if UserConfigDir fails
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	dir = filepath.Join(dir, "synca")
	return dir, os.MkdirAll(dir, 0700)
}

func Load() (*Config, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "config.json")

	cfg := &Config{
		WSAddr:     "localhost:7373",
		WatchPaths: []string{},
		IgnoredFolders: []string{
			"node_modules",
			".git",
		},
		Proxy:       ProxySettings{Mode: ProxyModeNone, Type: ProxyTypeSOCKS, Port: "1080"},
		TokenFile:   filepath.Join(dir, "token.json"),
		CredFile:    filepath.Join(dir, "credentials.json"),
		ConflictDir: filepath.Join(dir, "conflicts"),
		LogLevel:    "info",
		configPath:  path,
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, cfg.Save()
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.configPath = path

	// Normalize persisted watch paths so sync logic can reliably resolve hierarchy.
	normalized := make([]string, 0, len(cfg.WatchPaths))
	seen := make(map[string]struct{}, len(cfg.WatchPaths))
	for _, p := range cfg.WatchPaths {
		np := normalizePath(p)
		if np == "" {
			continue
		}
		if _, ok := seen[np]; ok {
			continue
		}
		seen[np] = struct{}{}
		normalized = append(normalized, np)
	}
	cfg.WatchPaths = normalized
	cfg.IgnoredFolders = normalizeIgnoredFolders(cfg.IgnoredFolders)
	cfg.Proxy = NormalizeProxySettings(cfg.Proxy)

	// Ensure mode map is always initialised (backward compat with older configs)
	if cfg.WatchPathModes == nil {
		cfg.WatchPathModes = make(map[string]SyncMode)
	}
	return cfg, nil
}

func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.configPath, data, 0600)
}

func (c *Config) AddWatchPath(path string) {
	c.AddWatchPathWithMode(path, ModeTwoWay)
}

// AddWatchPathWithMode adds a path with a specific sync mode.
func (c *Config) AddWatchPathWithMode(path string, mode SyncMode) {
	path = normalizePath(path)
	if path == "" {
		return
	}
	for _, p := range c.WatchPaths {
		if p == path {
			return
		}
	}
	c.WatchPaths = append(c.WatchPaths, path)
	if c.WatchPathModes == nil {
		c.WatchPathModes = make(map[string]SyncMode)
	}
	c.WatchPathModes[path] = mode
}

func (c *Config) RemoveWatchPath(path string) {
	path = normalizePath(path)
	filtered := c.WatchPaths[:0]
	for _, p := range c.WatchPaths {
		if p != path {
			filtered = append(filtered, p)
		}
	}
	c.WatchPaths = filtered
	delete(c.WatchPathModes, path)
}

// GetWatchPathMode returns the sync mode for a given watch path.
// Defaults to ModeTwoWay if not explicitly set (backward compat).
func (c *Config) GetWatchPathMode(path string) SyncMode {
	if c.WatchPathModes == nil {
		return ModeTwoWay
	}
	mode, ok := c.WatchPathModes[path]
	if !ok {
		return ModeTwoWay
	}
	return mode
}

// SetWatchPathMode updates the sync mode for a watch path.
func (c *Config) SetWatchPathMode(path string, mode SyncMode) {
	if c.WatchPathModes == nil {
		c.WatchPathModes = make(map[string]SyncMode)
	}
	c.WatchPathModes[path] = mode
}

func (c *Config) SetIgnoredFolders(patterns []string) {
	c.IgnoredFolders = normalizeIgnoredFolders(patterns)
}

func (c *Config) SetProxySettings(proxy ProxySettings) error {
	normalized := NormalizeProxySettings(proxy)
	if err := ValidateProxySettings(normalized); err != nil {
		return err
	}
	c.Proxy = normalized
	return nil
}

func NormalizeProxySettings(proxy ProxySettings) ProxySettings {
	proxy.Mode = ProxyMode(strings.TrimSpace(string(proxy.Mode)))
	proxy.Type = ProxyType(strings.TrimSpace(string(proxy.Type)))
	proxy.Host = strings.TrimSpace(proxy.Host)
	proxy.Port = strings.TrimSpace(proxy.Port)
	proxy.Username = strings.TrimSpace(proxy.Username)

	switch proxy.Mode {
	case ProxyModeSystem, ProxyModeManual:
	default:
		proxy.Mode = ProxyModeNone
	}

	switch proxy.Type {
	case ProxyTypeHTTP, ProxyTypeSOCKS:
	default:
		proxy.Type = ProxyTypeSOCKS
	}

	if proxy.Port == "" {
		if proxy.Type == ProxyTypeHTTP {
			proxy.Port = "8080"
		} else {
			proxy.Port = "1080"
		}
	}
	if proxy.Mode == ProxyModeManual && strings.Contains(proxy.Host, "://") {
		if u, err := url.Parse(proxy.Host); err == nil {
			if u.Hostname() != "" {
				proxy.Host = u.Hostname()
			}
			if u.Port() != "" {
				proxy.Port = u.Port()
			}
			if proxy.Username == "" && u.User != nil {
				proxy.Username = u.User.Username()
				proxy.Password, _ = u.User.Password()
			}
		}
	} else if proxy.Mode == ProxyModeManual {
		if host, port, err := net.SplitHostPort(proxy.Host); err == nil {
			proxy.Host = strings.Trim(host, "[]")
			if port != "" {
				proxy.Port = port
			}
		}
	}
	if proxy.Mode != ProxyModeManual {
		proxy.Host = ""
		proxy.Username = ""
		proxy.Password = ""
	}
	return proxy
}

func ValidateProxySettings(proxy ProxySettings) error {
	if proxy.Mode != ProxyModeManual {
		return nil
	}
	if proxy.Host == "" {
		return fmt.Errorf("proxy host is required")
	}
	port, err := strconv.Atoi(proxy.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("proxy port must be between 1 and 65535")
	}
	if proxy.Type == ProxyTypeHTTP {
		if _, err := ManualHTTPProxyURL(proxy); err != nil {
			return err
		}
	} else if _, err := ManualSOCKSAddress(proxy); err != nil {
		return err
	}
	return nil
}

func ManualHTTPProxyURL(proxy ProxySettings) (*url.URL, error) {
	proxy = NormalizeProxySettings(proxy)
	host := proxy.Host
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy host: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid proxy host")
	}
	if u.Port() == "" {
		u.Host = net.JoinHostPort(u.Hostname(), proxy.Port)
	}
	if proxy.Username != "" {
		if proxy.Password != "" {
			u.User = url.UserPassword(proxy.Username, proxy.Password)
		} else {
			u.User = url.User(proxy.Username)
		}
	}
	return u, nil
}

func ManualSOCKSAddress(proxy ProxySettings) (string, error) {
	proxy = NormalizeProxySettings(proxy)
	host := proxy.Host
	if strings.Contains(host, "://") {
		u, err := url.Parse(host)
		if err != nil {
			return "", fmt.Errorf("invalid proxy host: %w", err)
		}
		host = u.Hostname()
		if u.Port() != "" {
			proxy.Port = u.Port()
		}
	}
	if host == "" {
		return "", fmt.Errorf("invalid proxy host")
	}
	return net.JoinHostPort(host, proxy.Port), nil
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~"+string(os.PathSeparator)) || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				path = home
			} else {
				path = filepath.Join(home, path[2:])
			}
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

func normalizeIgnoredFolders(patterns []string) []string {
	seen := make(map[string]struct{}, len(patterns))
	normalized := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		pattern = strings.Trim(pattern, `/\`)
		if pattern == "" {
			continue
		}
		pattern = filepath.Clean(pattern)
		if pattern == "." {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		normalized = append(normalized, pattern)
	}
	return normalized
}
