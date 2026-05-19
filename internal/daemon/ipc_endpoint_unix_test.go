//go:build !windows

package daemon

import (
	"os"
	"testing"
)

func TestListenIPC_socketMode0600(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "bolt-sock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	ln, err := listenIPC(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	info, err := os.Stat(SocketPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("socket mode = %o, want 0600", info.Mode().Perm())
	}
}
