package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	WSAddr         string              `json:"ws_addr"`
	WatchPaths     []string            `json:"watch_paths"`
	WatchPathModes map[string]SyncMode `json:"watch_path_modes,omitempty"`
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
		WSAddr:      "localhost:7373",
		WatchPaths:  []string{},
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
