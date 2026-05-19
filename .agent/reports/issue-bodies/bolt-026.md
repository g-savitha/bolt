## Description
As a bolt user, I want the long-lived daemon to NOT inherit my shell's secrets (`AWS_SECRET_ACCESS_KEY`, `GITHUB_TOKEN`, `OPENAI_API_KEY`, `SSH_AUTH_SOCK`, `KUBECONFIG`, …), so that any future RCE in the daemon cannot exfiltrate credentials I never intended to share with bolt.

`internal/daemon/spawn.go:43-50` spawns the daemon via `exec.Command(exe, "daemon", "--config-dir", configDir)` without setting `cmd.Env`. Go's `os/exec` defaults to `cmd.Env = os.Environ()` (full inherit). The bolt daemon then runs forever in the background carrying every secret loaded in the parent shell. The daemon does not use these env vars today — but if it is ever compromised (parser bug, TLS bug, supply-chain issue), an attacker reads `/proc/<pid>/environ` or `ps eauxw` and exfiltrates the lot.

## Why
- Defence in depth: not exploitable on its own, but multiplies the blast radius of any future daemon vulnerability.
- The daemon is long-lived (intended uptime: days to weeks); a snapshot of secrets from the spawning shell sticks with it for that entire window.
- Sources: Security review `SEC-3` (`.agent/reports/security-review.md` §New findings — High).

## Acceptance Criteria
- [ ] `internal/daemon/spawn.go` sets `cmd.Env` to an explicit allowlist:
  ```go
  cmd.Env = []string{
      "PATH=" + os.Getenv("PATH"),
      "HOME=" + os.Getenv("HOME"),
      "USER=" + os.Getenv("USER"),
      "LANG=" + os.Getenv("LANG"),
      "TZ=" + os.Getenv("TZ"),
      // Optional once observability lands: BOLT_LOG, BOLT_QLOG.
  }
  ```
- [ ] Empty values are NOT propagated as `"FOO="` entries — only set what the parent actually has.
- [ ] Unit / integration test spawns the daemon under a synthetic env containing `SHOULD_NOT_LEAK=secret`, then asserts the variable is absent from the child's environment (read via `/proc/<pid>/environ` on Linux, or via a test-only IPC introspection command).
- [ ] No behaviour regression: manual smoke `bolt init` / `bolt daemon` / `EnsureRunning` work as before; `ps Eww -p $(cat ~/.config/bolt/daemon.pid)` shows only the allow-listed variables.
- [ ] Code comment documents that adding a new entry to the allowlist is a deliberate decision and should be reviewed.

## Notes
- Files: `internal/daemon/spawn.go` (lines 43-50), `internal/daemon/spawn_test.go` (new).
- Independent of all other Phase-2 stories — no decision deps.
- Source story: `.agent/backlog/tasks.md` BOLT-026. Originating finding: `.agent/reports/security-review.md` SEC-3.
