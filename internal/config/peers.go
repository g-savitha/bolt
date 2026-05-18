package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	peersFileName = "peers.toml"
	peersVersion  = 1
)

// TrustLevel controls how bolt responds to incoming connections and file
// transfer requests from a specific peer.
type TrustLevel string

const (
	// TrustAlwaysAllow means bolt accepts files from this peer automatically,
	// with no prompt. The user has explicitly decided to trust this machine.
	TrustAlwaysAllow TrustLevel = "always-allow"

	// TrustAllowOnce means bolt prompts for each incoming file or connection.
	// This is the default for newly discovered peers — cautious but not blocking.
	TrustAllowOnce TrustLevel = "allow-once"

	// TrustBlock means bolt silently rejects all connections from this peer.
	TrustBlock TrustLevel = "block"
)

// PeerRecord stores everything bolt knows about a specific remote machine.
// The fingerprint is the primary key — nickname and IP can change, the
// fingerprint is derived from the keypair and is stable for the machine's lifetime.
type PeerRecord struct {
	// Fingerprint is the SHA-256 hash of the peer's Ed25519 public key,
	// formatted as colon-separated hex. This is the true identity.
	Fingerprint string `toml:"fingerprint"`

	// Nickname is the human-readable name the peer advertises. Display-only —
	// never used for identity decisions.
	Nickname string `toml:"nickname"`

	// LastKnownIP is the most recent IP address we connected to this peer at.
	// Used as a hint for reconnection; not authoritative.
	LastKnownIP string `toml:"last_known_ip"`

	// Trust controls how this peer's connections and file requests are handled.
	Trust TrustLevel `toml:"trust"`

	// FirstSeen is when we first accepted a connection from this peer.
	FirstSeen time.Time `toml:"first_seen"`

	// ManuallyAdded is true when the user explicitly ran 'bolt connect <ip>'
	// rather than discovering this peer via mDNS or relay.
	ManuallyAdded bool `toml:"manually_added"`
}

// peersFile is the on-disk structure for peers.toml.
type peersFile struct {
	Version int          `toml:"version"`
	Peers   []PeerRecord `toml:"peers"`
}

// PeerStore is the in-memory, goroutine-safe store of known peers.
// All reads and writes go through this struct — nothing accesses peers.toml directly.
// This ensures consistent state even when mDNS discovers multiple peers simultaneously.
type PeerStore struct {
	mu    sync.RWMutex
	dir   string
	peers map[string]*PeerRecord // keyed by fingerprint
}

// LoadPeerStore reads peers.toml from dir and returns a ready-to-use PeerStore.
// If the file does not exist, an empty store is returned and will be created
// on the first write.
func LoadPeerStore(dir string) (*PeerStore, error) {
	path := filepath.Join(dir, peersFileName)

	store := &PeerStore{
		dir:   dir,
		peers: make(map[string]*PeerRecord),
	}

	data, err := os.ReadFile(path) //nolint:gosec // G304: path is derived from DefaultConfigDir(), not user input
	if os.IsNotExist(err) {
		// No peers file yet — that's fine, we'll create it on first write.
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read peers file: %w", err)
	}

	var pf peersFile
	if _, err := toml.Decode(string(data), &pf); err != nil {
		return nil, fmt.Errorf("parse peers file at %s: %w", path, err)
	}

	if pf.Version > peersVersion {
		return nil, fmt.Errorf(
			"peers file version %d is newer than this bolt binary (version %d) — please update bolt",
			pf.Version, peersVersion,
		)
	}

	for i := range pf.Peers {
		p := pf.Peers[i]
		store.peers[p.Fingerprint] = &p
	}

	return store, nil
}

// Get returns the peer record for the given fingerprint, or nil if unknown.
func (s *PeerStore) Get(fingerprint string) *PeerRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peers[fingerprint]
}

// GetByNickname returns all peers whose nickname matches (case-sensitive).
// There may be more than one result — nicknames are not unique identifiers.
func (s *PeerStore) GetByNickname(nickname string) []*PeerRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []*PeerRecord
	for _, p := range s.peers {
		if p.Nickname == nickname {
			matches = append(matches, p)
		}
	}
	return matches
}

// All returns a snapshot of all known peers. The slice is safe to read
// without holding the lock — it's a copy.
func (s *PeerStore) All() []PeerRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PeerRecord, 0, len(s.peers))
	for _, p := range s.peers {
		result = append(result, *p)
	}
	return result
}

// Upsert adds a new peer or updates an existing one, then flushes to disk.
// The fingerprint field of record is used as the primary key.
func (s *PeerStore) Upsert(record PeerRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.Fingerprint == "" {
		return fmt.Errorf("peer record must have a non-empty fingerprint")
	}

	existing, exists := s.peers[record.Fingerprint]
	if exists {
		// Preserve FirstSeen — it is immutable once set.
		record.FirstSeen = existing.FirstSeen
	} else {
		if record.FirstSeen.IsZero() {
			record.FirstSeen = time.Now()
		}
	}

	s.peers[record.Fingerprint] = &record
	return s.flushLocked()
}

// SetTrust updates only the trust level for an existing peer and flushes to disk.
// Returns an error if the fingerprint is not known.
func (s *PeerStore) SetTrust(fingerprint string, trust TrustLevel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[fingerprint]
	if !ok {
		return fmt.Errorf("unknown peer %s — add this peer first with 'bolt connect'", fingerprint)
	}

	peer.Trust = trust
	return s.flushLocked()
}

// flushLocked writes the current in-memory state to peers.toml.
// Caller must hold s.mu (write lock).
func (s *PeerStore) flushLocked() error {
	pf := peersFile{
		Version: peersVersion,
		Peers:   make([]PeerRecord, 0, len(s.peers)),
	}
	for _, p := range s.peers {
		pf.Peers = append(pf.Peers, *p)
	}

	path := filepath.Join(s.dir, peersFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644) //nolint:gosec // G304: path is derived from DefaultConfigDir(), not user input
	if err != nil {
		return fmt.Errorf("open peers file for writing: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(pf); err != nil {
		return fmt.Errorf("encode peers to toml: %w", err)
	}
	return nil
}
