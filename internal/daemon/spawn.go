package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

// daemonEnvKeys lists environment variables the spawned daemon may inherit.
// Adding a key is a deliberate security decision — review before extending.
var (
	daemonEnvAlways = []string{"PATH", "HOME", "USER", "LANG", "TZ", "TMPDIR"}
	daemonEnvIfSet  = []string{
		"BOLT_LOG", "BOLT_QLOG",
		"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR",
	}
)

const (
	pidFile        = "daemon.pid"
	spawnPollDelay = 50 * time.Millisecond
	spawnTimeout   = 3 * time.Second
)

func pidFilePath(configDir string) string {
	return filepath.Join(configDir, pidFile)
}

// writeDaemonPID records the running daemon PID after the listener is up.
func writeDaemonPID(configDir string) error {
	pidPath := pidFilePath(configDir)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data := []byte(strconv.Itoa(os.Getpid()) + "\n")
	return os.WriteFile(pidPath, data, 0600) //nolint:gosec // G306: path from config dir
}

func removeDaemonPID(configDir string) error {
	err := os.Remove(pidFilePath(configDir))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
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

	cmd := exec.Command(exe, "daemon", "--config-dir", configDir) //nolint:gosec // exe is resolved via os.Executable(), not user input
	cmd.Env = daemonEnv(os.Environ())
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	setDetached(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon process: %w", err)
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

func daemonEnv(parent []string) []string {
	lookup := envMap(parent)
	var out []string
	for _, key := range daemonEnvAlways {
		if v, ok := lookup[key]; ok {
			out = append(out, key+"="+v)
		}
	}
	for _, key := range daemonEnvIfSet {
		if v, ok := lookup[key]; ok && v != "" {
			out = append(out, key+"="+v)
		}
	}
	return out
}

func envMap(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, entry := range environ {
		key, val, ok := strings.Cut(entry, "=")
		if ok {
			m[key] = val
		}
	}
	return m
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
