package daemon

import (
	"os"
	"strconv"
	"testing"
)

func TestDaemonPIDWriteAndRemove(t *testing.T) {
	dir := t.TempDir()

	if err := writeDaemonPID(dir); err != nil {
		t.Fatal(err)
	}
	path := pidFilePath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strconv.Itoa(os.Getpid()) + "\n"
	if string(data) != want {
		t.Fatalf("pid file = %q, want %q", data, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o, want 0600", info.Mode().Perm())
	}

	if err := removeDaemonPID(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("pid file should be removed, stat err=%v", err)
	}
}
