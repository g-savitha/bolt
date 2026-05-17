package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const ipcTokenFile = "ipc.token"

// loadOrCreateIPCToken reads the daemon IPC auth token from disk, or creates one.
// The token must be presented by every CLI connection so other local users cannot
// control the daemon.
func loadOrCreateIPCToken(configDir string) (string, error) {
	path := filepath.Join(configDir, ipcTokenFile)

	data, err := os.ReadFile(path)
	if err == nil {
		token := string(data)
		if len(token) >= 16 {
			return trimNewline(token), nil
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read ipc token: %w", err)
	}

	token, err := generateIPCToken()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
		return "", fmt.Errorf("write ipc token: %w", err)
	}
	return token, nil
}

// readIPCToken loads the token written by the daemon. Used by CLI clients.
func readIPCToken(configDir string) (string, error) {
	path := filepath.Join(configDir, ipcTokenFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf(
				"ipc token not found — is the bolt daemon running? Try: bolt init",
			)
		}
		return "", fmt.Errorf("read ipc token: %w", err)
	}
	token := trimNewline(string(data))
	if len(token) < 16 {
		return "", fmt.Errorf("ipc token file is corrupt — restart the daemon with: bolt daemon")
	}
	return token, nil
}

func generateIPCToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate ipc token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
