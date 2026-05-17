package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofrs/flock"
)

const (
	pidFile        = "daemon.pid"
	spawnPollDelay = 50 * time.Millisecond
	spawnTimeout   = 3 * time.Second
)

func pidFilePath(configDir string) string {
	return filepath.Join(configDir, pidFile)
}

// EnsureRunning starts the daemon if it is not already listening.
func EnsureRunning(configDir string) error {
	if isDaemonReachable(configDir) {
		return nil
	}
	return withPIDLock(configDir, func() error {
		if isDaemonReachable(configDir) {
			return nil
		}
		return spawnDaemon(configDir)
	})
}

func spawnDaemon(configDir string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate bolt executable: %w", err)
	}

	cmd := exec.Command(exe, "daemon", "--config-dir", configDir)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	setDetached(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon process: %w", err)
	}

	pidPath := pidFilePath(configDir)
	pidData := strconv.Itoa(cmd.Process.Pid) + "\n"
	if err := os.WriteFile(pidPath, []byte(pidData), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write daemon pid file: %v\n", err)
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(spawnTimeout)
	for time.Now().Before(deadline) {
		if isDaemonReachable(configDir) {
			return nil
		}
		time.Sleep(spawnPollDelay)
	}
	return fmt.Errorf(
		"daemon did not start within %s — try: bolt daemon",
		spawnTimeout,
	)
}

func withPIDLock(configDir string, fn func() error) error {
	pidPath := pidFilePath(configDir)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	fl := flock.New(pidPath)
	if err := fl.Lock(); err != nil {
		return fmt.Errorf("acquire pid file lock: %w", err)
	}
	defer fl.Unlock() //nolint:errcheck

	return fn()
}
