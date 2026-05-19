package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestWriteAtomicPreservesOriginalOnEncodeFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.toml")
	original := []byte("version = 1\npeers = []\n")
	if err := os.WriteFile(path, original, privateFileMode); err != nil {
		t.Fatal(err)
	}

	err := writeAtomic(path, func(w io.Writer) error {
		if _, err := w.Write([]byte("version = 1\npeers = [\n")); err != nil {
			return err
		}
		return errors.New("simulated encode failure")
	})
	if err == nil {
		t.Fatal("expected encode error")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("original file changed:\ngot:  %q\nwant: %q", got, original)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file should be removed, stat err=%v", err)
	}
}

func TestWriteAtomicNeverLeavesEmptyOrPartialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	original := []byte("version = 1\nnickname = \"alice\"\n")
	if err := os.WriteFile(path, original, privateFileMode); err != nil {
		t.Fatal(err)
	}

	const attempts = 20
	for i := 0; i < attempts; i++ {
		failAfter := 5 + (i % 40)
		_ = writeAtomic(path, func(w io.Writer) error {
			payload := strings.Repeat("x", 200)
			if _, err := w.Write([]byte(payload[:failAfter])); err != nil {
				return err
			}
			return errors.New("simulated crash mid-write")
		})

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) == 0 {
			t.Fatal("file must never be empty after failed write")
		}
		s := string(data)
		if s != string(original) {
			if len(s) < len(original) {
				t.Fatalf("partial/truncated file (%d bytes): %q", len(s), s)
			}
		}
	}

	if err := writeAtomic(path, func(w io.Writer) error {
		_, err := w.Write([]byte("version = 1\nnickname = \"bob\"\n"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "version = 1\nnickname = \"bob\"\n" {
		t.Fatalf("final write failed: %q", got)
	}
}

func TestWriteAtomicFileMode0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := writeAtomic(path, func(w io.Writer) error {
		_, err := w.Write([]byte("version = 1\n"))
		return err
	}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != privateFileMode {
		t.Fatalf("mode = %o, want %o", info.Mode().Perm(), privateFileMode)
	}
}

func TestPeerStoreConcurrentUpsertRace(t *testing.T) {
	dir := t.TempDir()
	store, err := LoadPeerStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			fp := fmt.Sprintf("fp%064d", i)
			if err := store.Upsert(PeerRecord{
				Fingerprint: fp,
				Nickname:    "peer",
				Trust:       TrustAllowOnce,
			}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	reloaded, err := LoadPeerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.All()) != n {
		t.Fatalf("got %d peers, want %d", len(reloaded.All()), n)
	}

	path := filepath.Join(dir, peersFileName)
	var pf peersFile
	if _, err := toml.DecodeFile(path, &pf); err != nil {
		t.Fatal(err)
	}
	if pf.Version != peersVersion {
		t.Fatalf("version = %d", pf.Version)
	}
}
