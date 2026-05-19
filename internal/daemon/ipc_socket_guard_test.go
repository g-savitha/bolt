//go:build !windows

package daemon

import (
	"os"
	"testing"

	"github.com/bolt/bolt/internal/config"
	"github.com/bolt/bolt/internal/identity"
)

func TestListenIPC_rejectsWhenDaemonAlreadyRunning(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "bolt-guard-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	id, err := identity.Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Nickname: "test", Port: 7798}
	peers, err := config.LoadPeerStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	d := New(id, cfg, peers, dir)
	srv, err := NewIPCServer(dir, d)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })

	_, err = listenIPC(dir)
	if err == nil {
		t.Fatal("expected error when socket is held by a running daemon")
	}
}
