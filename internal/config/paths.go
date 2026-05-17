package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// AppName is the CLI and config directory name.
	AppName = "bolt"
)

// DefaultConfigDir returns ~/.config/bolt.
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("bolt: home directory: %v", err))
	}
	return filepath.Join(home, ".config", AppName)
}
