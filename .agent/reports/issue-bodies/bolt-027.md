## Description
As a bolt user on a multi-user host, I want `bolt init` to refuse to run if `~/.config/bolt/` exists with mode wider than `0700`, and I want `private.key` / `ipc.token` / `daemon.pid` to never be written through an attacker-planted symlink, so that a low-privilege local attacker cannot redirect bolt's first-time writes onto `~/.ssh/id_ed25519` or any other file I can write to.

Today every write in the config / identity / token / pid paths uses `os.WriteFile` or `os.OpenFile(... O_CREATE ...)` without `O_NOFOLLOW`. Go's `os.WriteFile` follows symlinks. Also: `os.MkdirAll(dir, 0700)` does NOT change permissions on a directory that already exists with different mode — if `~/.config/bolt/` was created earlier with 0755 (e.g. by a hostile parent shell), `MkdirAll` returns success and the bolt daemon happily runs with a world-traversable config dir.

Manual reproduction (DO NOT run on a machine with a real bolt identity):
```
mkdir -p ~/.config/bolt && chmod 755 ~/.config/bolt
ln -s /tmp/pwn ~/.config/bolt/private.key
rm -rf ~/.config/bolt/identity
bolt init   # writes private.key through the symlink to /tmp/pwn
```

## Why
- Requires a pre-positioned local attacker, but the blast radius is destruction or redirection of the user's identity key.
- Cheap to fix: one extra `O_NOFOLLOW|O_EXCL` flag on first writes and one `os.Stat` after `MkdirAll`.
- Sources: Security review `SEC-6` (`.agent/reports/security-review.md` §New findings — Medium).

## Acceptance Criteria
- [ ] After `os.MkdirAll(dir, 0700)` in `internal/config/config.go` and `peers.go`, call `os.Stat` and error out if `info.Mode().Perm() & 0o077 != 0` — explicit error names the offending path and required mode.
- [ ] First-write of `private.key` (`internal/identity/identity.go:206`), `ipc.token` (`internal/daemon/ipc_token.go:38`), and `daemon.pid` (post-BOLT-005, owned by `Daemon.Run`) uses `os.OpenFile(path, O_WRONLY|O_CREATE|O_EXCL|O_NOFOLLOW, 0600)`.
- [ ] `writeAtomic` from BOLT-003 opens its `*.tmp` file with `O_EXCL|O_NOFOLLOW|O_CREATE|O_WRONLY` so the temp file cannot be redirected through a pre-planted symlink either.
- [ ] Unit test: pre-create `~/.config/bolt/private.key` as a symlink to a sentinel path, run `Identity.Generate`, assert it errors (does NOT write through the symlink) and the sentinel is unchanged.
- [ ] Unit test: pre-create `~/.config/bolt/` with mode 0755, run `LoadOrInit`, assert it errors with a message naming the directory and "0700".
- [ ] Documented in `architecture.md` (or follow-up `SECURITY.md`) that the config dir must be `0700` and bolt enforces this.

## Notes
- Files: `internal/identity/identity.go`, `internal/daemon/ipc_token.go`, `internal/daemon/daemon.go` (PID write post-BOLT-005), `internal/config/{config.go, peers.go, atomic.go}`.
- Depends on BOLT-003 (for the atomic-write temp-file flag change) and BOLT-005 (for the PID write moving into `Daemon.Run`). P2 — schedule after the BOLT-003/005 work lands.
- `O_NOFOLLOW` and `O_EXCL` are honoured on both Linux and macOS.
- Source story: `.agent/backlog/tasks.md` BOLT-027. Originating finding: `.agent/reports/security-review.md` SEC-6.
