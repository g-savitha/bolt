package daemon

import (
	"slices"
	"strings"
	"testing"
)

func TestDaemonEnvAllowlistExcludesSecrets(t *testing.T) {
	parent := []string{
		"PATH=/usr/bin",
		"HOME=/home/alice",
		"USER=alice",
		"LANG=en_US.UTF-8",
		"AWS_SECRET_ACCESS_KEY=secret",
		"GITHUB_TOKEN=ghp_test",
		"SHOULD_NOT_LEAK=secret",
		"BOLT_LOG=json",
	}

	got := daemonEnv(parent)
	for _, entry := range got {
		key, _, _ := strings.Cut(entry, "=")
		switch {
		case strings.HasPrefix(key, "AWS_"),
			strings.HasPrefix(key, "GITHUB_"),
			key == "SHOULD_NOT_LEAK":
			t.Fatalf("forbidden env leaked into daemon: %s", entry)
		}
	}

	if !slices.Contains(got, "PATH=/usr/bin") {
		t.Fatalf("PATH missing from %v", got)
	}
	if !slices.Contains(got, "BOLT_LOG=json") {
		t.Fatalf("BOLT_LOG missing from %v", got)
	}
}

func TestDaemonEnvSkipsEmptyOptional(t *testing.T) {
	parent := []string{
		"PATH=/bin",
		"HOME=/h",
		"USER=u",
		"BOLT_LOG=",
	}
	got := daemonEnv(parent)
	for _, entry := range got {
		if strings.HasPrefix(entry, "BOLT_LOG=") {
			t.Fatalf("empty optional var should be omitted, got %q", entry)
		}
	}
}
