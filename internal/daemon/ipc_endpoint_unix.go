//go:build !windows

package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

const socketFile = "daemon.sock"

// SocketPath returns the Unix socket path for CLI↔daemon IPC on this platform.
func SocketPath(configDir string) string {
	return filepath.Join(configDir, socketFile)
}

func listenIPC(configDir string) (net.Listener, error) {
	socketPath := SocketPath(configDir)
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale socket: %w", err)
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen on unix socket %s: %w", socketPath, err)
	}
	return ln, nil
}

func dialIPC(configDir string) (net.Conn, error) {
	conn, err := net.Dial("unix", SocketPath(configDir))
	if err != nil {
		return nil, fmt.Errorf(
			"connect to bolt daemon: %w\n  Is the daemon running? Try: bolt init",
			err,
		)
	}
	return conn, nil
}

func cleanupIPC(configDir string) {
	_ = os.Remove(SocketPath(configDir))
}

func isDaemonReachable(configDir string) bool {
	conn, err := dialIPC(configDir)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
