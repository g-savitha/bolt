package daemon

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const (
	pidFile        = "daemon.pid"
	socketFile     = "daemon.sock"
	spawnPollDelay = 50 * time.Millisecond
	spawnTimeout   = 3 * time.Second
)

// SocketPath returns the path of the Unix socket for the given config directory.
func SocketPath(configDir string) string {
	return filepath.Join(configDir, socketFile)
}

// pidFilePath returns the path of the PID file for the given config directory.
func pidFilePath(configDir string) string {
	return filepath.Join(configDir, pidFile)
}

// EnsureRunning checks if a daemon is already running in configDir.
// If not, it spawns one as a detached background process and waits until
// the Unix socket is ready to accept connections.
//
// Uses a flock on the PID file to prevent two CLI invocations racing to
// spawn duplicate daemons.
func EnsureRunning(configDir string) error {
	socketPath := SocketPath(configDir)

	// Fast path: daemon is already running and reachable.
	if isDaemonReachable(socketPath) {
		return nil
	}

	// Slow path: acquire an exclusive lock on the PID file, check again,
	// then spawn if still not running.
	return withPIDLock(configDir, func() error {
		// Re-check inside the lock — another process may have spawned
		// the daemon between our first check and acquiring the lock.
		if isDaemonReachable(socketPath) {
			return nil
		}
		return spawnDaemon(configDir)
	})
}

// isDaemonReachable tries to connect to the Unix socket.
// Returns true if a daemon is listening, false otherwise.
func isDaemonReachable(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// spawnDaemon launches the current executable with the "daemon" subcommand
// as a detached subprocess (new session, stdio connected to /dev/null).
// It then waits for the Unix socket to become reachable.
func spawnDaemon(configDir string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate flick executable: %w", err)
	}

	cmd := exec.Command(exe, "daemon", "--config-dir", configDir)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Setsid creates a new session, detaching the child from this terminal.
	// Without this, the child would receive signals sent to the parent's
	// process group (e.g. Ctrl+C in the terminal would kill it).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon process: %w", err)
	}

	// Write the child's PID so we can check liveness later.
	pidPath := pidFilePath(configDir)
	pidData := strconv.Itoa(cmd.Process.Pid) + "\n"
	if err := os.WriteFile(pidPath, []byte(pidData), 0644); err != nil {
		// Non-fatal — the daemon is running, we just can't track its PID.
		fmt.Fprintf(os.Stderr, "warning: could not write daemon pid file: %v\n", err)
	}

	// Detach from the child — we don't want to wait for it.
	_ = cmd.Process.Release()

	return waitForSocket(SocketPath(configDir))
}

// waitForSocket polls the Unix socket path until a daemon is listening or
// the timeout is exceeded.
func waitForSocket(socketPath string) error {
	deadline := time.Now().Add(spawnTimeout)

	for time.Now().Before(deadline) {
		if isDaemonReachable(socketPath) {
			return nil
		}
		time.Sleep(spawnPollDelay)
	}

	return fmt.Errorf(
		"daemon did not start within %s — check logs or try running 'flick daemon' manually",
		spawnTimeout,
	)
}

// withPIDLock acquires an exclusive flock on the PID file, runs fn, then
// releases the lock. This ensures at most one daemon is spawned even if
// multiple CLI invocations race at startup.
func withPIDLock(configDir string, fn func() error) error {
	pidPath := pidFilePath(configDir)

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	f, err := os.OpenFile(pidPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open pid file: %w", err)
	}
	defer f.Close()

	// Exclusive lock — blocks if another process holds it.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire pid file lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}
