//go:build windows

package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const ipcPortFile = "ipc.port"

// SocketPath is not used on Windows; CLI dials the TCP address from ipc.port.
func SocketPath(configDir string) string {
	return ipcAddrPath(configDir)
}

func ipcAddrPath(configDir string) string {
	return filepath.Join(configDir, ipcPortFile)
}

func listenIPC(configDir string) (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen ipc on loopback: %w", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port
	if err := os.MkdirAll(configDir, 0700); err != nil {
		ln.Close()
		return nil, err
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := os.WriteFile(ipcAddrPath(configDir), []byte(addr+"\n"), 0600); err != nil {
		ln.Close()
		return nil, fmt.Errorf("write ipc address file: %w", err)
	}
	return ln, nil
}

func dialIPC(configDir string) (net.Conn, error) {
	addr, err := readIPCAddr(configDir)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf(
			"connect to bolt daemon at %s: %w\n  Is the daemon running? Try: bolt init",
			addr, err,
		)
	}
	return conn, nil
}

func readIPCAddr(configDir string) (string, error) {
	data, err := os.ReadFile(ipcAddrPath(configDir))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("daemon ipc address not found — is bolt running? Try: bolt init")
		}
		return "", fmt.Errorf("read ipc address: %w", err)
	}
	addr := strings.TrimSpace(string(data))
	if _, err := strconv.Atoi(strings.TrimPrefix(addr, "127.0.0.1:")); err != nil {
		return "", fmt.Errorf("invalid ipc address in %s", ipcAddrPath(configDir))
	}
	return addr, nil
}

func cleanupIPC(configDir string) {
	_ = os.Remove(ipcAddrPath(configDir))
}

func isDaemonReachable(configDir string) bool {
	conn, err := dialIPC(configDir)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
