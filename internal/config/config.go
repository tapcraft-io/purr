package config

import (
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	// Preferences
	DefaultNamespace    string
	HistorySize         int
	CacheTTL            int
	ConfirmDestructive  bool

	// UI
	Theme               string
	ShowHelp            bool
	CompactMode         bool

	// Paths
	ConfigDir           string
	HistoryFile         string

	// Kubernetes
	KubeconfigPath      string
}

// NewConfig creates a new configuration with defaults
func NewConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".purr")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	}

	return &Config{
		DefaultNamespace:   "default",
		HistorySize:        1000,
		CacheTTL:           30,
		ConfirmDestructive: true,
		Theme:              "dark",
		ShowHelp:           true,
		CompactMode:        false,
		ConfigDir:          configDir,
		HistoryFile:        filepath.Join(configDir, "history.json"),
		KubeconfigPath:     kubeconfigPath,
	}, nil
}
