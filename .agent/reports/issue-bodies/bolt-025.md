## Description
As a bolt user, I want peer-supplied nicknames and chat bodies to be stripped of ANSI / CSI / OSC escape sequences before they hit my terminal or get persisted to `peers.toml`, so that a malicious peer cannot clear my scrollback, set my terminal title, forge prior chat lines via `\r`, hide invisible text with `\x1b[8m`, or hijack my clipboard via OSC-52 on macOS Terminal.app.

Today every peer-controlled string (`HandshakeMsg.Nickname`, `ChatMsg.From`, `ChatMsg.Body`) is printed to a real terminal — daemon stderr and chat CLI stdout — via `fmt.Fprintf` / `fmt.Printf` with no filtering. Concrete attacks on the current code:
- Nickname `\x1b]2;evil\x07` (OSC-2 set-title) — changes the user's terminal window title.
- Nickname `\x1b[2J\x1b[H` (clear-screen + cursor-home) — wipes chat scrollback.
- Chat body containing `\r` — overwrites the previous chat line, forging prior messages from another peer in scrollback view.
- `\x1b[8m`-wrapped text — sneaks invisible content into chat scrollback that copies/pastes later.
- On macOS Terminal.app: `\x1b]52;c;...\x07` (OSC-52) sets the system clipboard from a chat message.

`encoding/json` escapes control bytes on marshal but `json.Unmarshal` reconstitutes them, so the raw escape bytes are back in the string by the time the CLI subscriber prints them.

## Why
- Combined with `SEC-7` (nickname confusion), a malicious peer can impersonate a trusted contact persuasively in chat scrollback.
- Clipboard hijack and terminal title spoofing are real attacks against any user running `bolt chat` from a stock terminal.
- Sources: Security review `SEC-2` (`.agent/reports/security-review.md` §New findings — High).

## Acceptance Criteria
- [ ] New helper `func SanitizeDisplay(s string, maxRunes int) string` in `internal/proto/sanitize.go` replaces every byte `< 0x20` (except `\n`, `\t`) and `\x7f` with the literal escaped form (`\x1b` → `\\x1b`) and truncates to `maxRunes`.
- [ ] Called on **receive** in `internal/transport/peer_conn.go::validateHandshake` (nickname; cap 64 runes) before assigning to `pc.peerNickname`.
- [ ] Called on **receive** in `internal/chat/service.go::AttachStream` for `msg.From` and `msg.Body` (cap From at 64 runes, Body at 4 KiB) before passing into `s.emit`.
- [ ] Called on **display** in `cmd/bolt/main.go::chatCmd` before `fmt.Printf` of `payload.From` and `payload.Body` (defence in depth — if the daemon ever forgets, the CLI still scrubs).
- [ ] Unit test: nickname containing `"alice\x1b[2J\x1b[H"` round-trips through the daemon as `"alice\\x1b[2J\\x1b[H"`; chat body containing `\r` is escaped, not rendered.
- [ ] Wire bytes are NOT sanitized at marshal — keep the wire faithful for forensic logs; sanitize on display only.

## Notes
- Files: `internal/proto/sanitize.go` (new), `internal/proto/sanitize_test.go` (new), `internal/transport/peer_conn.go` (line ~137), `internal/chat/service.go`, `cmd/bolt/main.go` (line ~281), `internal/daemon/daemon.go` (lines 188, 190, 210 — the `fmt.Fprintf` sites that log peer nicknames).
- Independent of all other Phase-2 stories. Pair with BOLT-001 so the TOFU prompt itself safely renders peer-supplied nicknames + fingerprints.
- Source story: `.agent/backlog/tasks.md` BOLT-025. Originating finding: `.agent/reports/security-review.md` SEC-2.
