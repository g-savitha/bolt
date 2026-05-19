## Description
Both `flushLocked` (`internal/config/peers.go:193-217`) and `write` (`internal/config/config.go:178-193`) open the destination with `O_WRONLY|O_CREATE|O_TRUNC` and stream a TOML encoder directly into the file. There is no temp-file + `Rename`, no `Sync`. A crash, `kill -9`, OOM, full disk, or power loss between truncate and encode-complete leaves the user with a half-written or zero-byte `peers.toml`. Trust state is the most security-relevant on-disk state in the project — losing it forces every peer back to TOFU, and (combined with BUG-1) silently re-accepts everyone the user previously trusted.

## Steps to Reproduce
1. Inspect `internal/config/peers.go::flushLocked` and `internal/config/config.go::write`.
2. Confirm there is no `os.CreateTemp` / `os.Rename` pair.
3. (Optional dynamic) Inject an `io.Writer` wrapper that returns an error after N bytes; observe the file truncated to N bytes after the call returns.

## Expected Behavior
Atomic write — encode to `peers.toml.tmp` in the same directory, `f.Sync()`, `f.Close()`, then `os.Rename` over the destination. On Linux + ext4 default this is crash-atomic; on macOS APFS, rename is atomic at the directory-entry level.

## Actual Behavior
Truncate-then-encode. Crash mid-write erases or truncates the file.

## Environment
- `go version go1.25.1 darwin/arm64`
- `BurntSushi/toml v1.6.0`
- macOS 25.2.0
- Repo at `e267dd2`

## Suggested Fix
Single helper `writeAtomic(path string, fn func(io.Writer) error) error` in `internal/config/` reused by both `peers.go` and `config.go`. Bonus: unit test that injects a writer wrapper which fails after N bytes and asserts the original file is untouched. Tracked as story BOLT-003.
