// Package config manages flick's on-disk configuration.
//
// There are two configuration files:
//   - config.toml: user preferences (nickname, ports, paths, relay settings)
//   - peers.toml:  known peers and their trust levels
//
// Both files carry a version field so future versions of flick can migrate
// old configs forward rather than breaking on unexpected schemas.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	configFileName = "config.toml"
	configVersion  = 1

	// DefaultPort is the UDP port flick listens on for QUIC connections.
	DefaultPort = 7799

	// DefaultReceiveDir is where received files land when no override is set.
	// We use ~/Downloads because it's familiar to users on both Linux and macOS.
	DefaultReceiveDir = "~/Downloads"
)

// Config holds user-facing preferences for a flick node.
// All fields have sensible defaults so a freshly generated config works
// without the user needing to edit anything.
type Config struct {
	// Version allows future migrations. Always written as configVersion.
	Version int `toml:"version"`

	// Nickname is the human-readable name for this machine shown to peers.
	// Defaults to the system hostname.
	Nickname string `toml:"nickname"`

	// Port is the UDP port for QUIC connections. Default: 7799.
	Port int `toml:"port"`

	// ReceiveDir is where incoming files are saved. Default: ~/Downloads.
	ReceiveDir string `toml:"receive_dir"`

	// LogChat enables opt-in chat and transfer logging to
	// ~/.local/share/flick/logs/. Off by default (ephemeral by design).
	LogChat bool `toml:"log_chat"`

	// LogTransfers enables opt-in transfer history logging alongside chat logs.
	LogTransfers bool `toml:"log_transfers"`

	// Relay is the URL of the flick relay server for internet peer discovery.
	// Empty means LAN-only mode.
	Relay string `toml:"relay"`

	// RelayCertFingerprint pins the relay server's TLS certificate fingerprint.
	// Required when the relay uses a self-signed certificate (IP-only deployment).
	RelayCertFingerprint string `toml:"relay_cert_fingerprint"`

	// MaxRelayTransferBytes caps file transfers that flow through TURN relay.
	// 0 means no limit. Default: 5 GB.
	MaxRelayTransferBytes int64 `toml:"max_relay_transfer_bytes"`

	// Stealth suppresses both mDNS broadcasting and relay registration.
	// When true, this machine is only reachable via direct 'flick connect <ip>'.
	Stealth bool `toml:"stealth"`
}

// defaults returns a Config with all fields set to their initial values.
// Called when generating a new config or migrating an old one.
func defaults(hostname string) Config {
	return Config{
		Version:               configVersion,
		Nickname:              hostname,
		Port:                  DefaultPort,
		ReceiveDir:            DefaultReceiveDir,
		LogChat:               false,
		LogTransfers:          false,
		Relay:                 "",
		RelayCertFingerprint:  "",
		MaxRelayTransferBytes: 5 * 1024 * 1024 * 1024, // 5 GB
		Stealth:               false,
	}
}

// Load reads config.toml from dir. If the file does not exist, Load creates
// it with default values. If the file's version is older than the current
// version, missing fields are filled with defaults and the file is rewritten.
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, configFileName)

	// If no config file exists yet, write one with defaults and return.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg, err := newWithDefaults()
		if err != nil {
			return nil, err
		}
		if err := write(path, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file at %s: %w", path, err)
	}

	// Migrate if the config predates the current version.
	changed, err := migrate(&cfg)
	if err != nil {
		return nil, err
	}
	if changed {
		if err := write(path, &cfg); err != nil {
			return nil, fmt.Errorf("write migrated config: %w", err)
		}
	}

	return &cfg, nil
}

// Save writes cfg to config.toml in dir, overwriting any existing file.
func Save(dir string, cfg *Config) error {
	return write(filepath.Join(dir, configFileName), cfg)
}

// migrate fills in missing or zero-valued fields with defaults for the current
// version. Returns true if any field was changed (caller should rewrite file).
func migrate(cfg *Config) (changed bool, err error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	d := defaults(hostname)

	if cfg.Version == 0 {
		cfg.Version = configVersion
		changed = true
	}
	if cfg.Version > configVersion {
		return false, fmt.Errorf(
			"config file version %d is newer than this flick binary (version %d) — please update flick",
			cfg.Version, configVersion,
		)
	}
	if cfg.Nickname == "" {
		cfg.Nickname = d.Nickname
		changed = true
	}
	if cfg.Port == 0 {
		cfg.Port = d.Port
		changed = true
	}
	if cfg.ReceiveDir == "" {
		cfg.ReceiveDir = d.ReceiveDir
		changed = true
	}
	if cfg.MaxRelayTransferBytes == 0 {
		cfg.MaxRelayTransferBytes = d.MaxRelayTransferBytes
		changed = true
	}
	return changed, nil
}

func newWithDefaults() (*Config, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	cfg := defaults(hostname)
	return &cfg, nil
}

func write(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open config file for writing: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("encode config to toml: %w", err)
	}
	return nil
}
