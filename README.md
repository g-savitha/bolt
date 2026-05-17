# bolt

Fast, encrypted, terminal-native P2P chat and file transfer between machines you control.

No accounts. No central servers. No internet required for LAN use. Your Ed25519 keypair is your identity.

---

## What it does

- **Send files** between your machines at wire speed over LAN, or across the internet via your own relay
- **Chat** in real time from the terminal — 1:1 or group
- **Auto-discover** peers on the same network with zero configuration
- **Authenticate** every peer cryptographically — TOFU model, same as SSH
- **Encrypt** everything with TLS 1.3 over QUIC — no plaintext ever on the wire

## What it is not

- Not a cloud service — you own all the infrastructure
- Not anonymous — every connection is tied to a cryptographic fingerprint
- Not moderated — designed for machines you control and people you trust

---

## Install

See **[INSTALL.md](INSTALL.md)** for Homebrew, Scoop, and release binaries.

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/g-savitha/bolt/main/scripts/install.sh | bash

# or from source
git clone https://github.com/g-savitha/bolt.git
cd bolt && go build -o bolt ./cmd/bolt
```

**Supported platforms:** macOS, Linux, Windows (amd64). Single static binary, no runtime deps.

**Open source client (this repo).** Paid internet relay (`boltd` on your VPS) is separate — no relay secrets here.

---

## Quick start

```bash
bolt init
bolt connect 192.168.1.42   # peer IP on same WiFi
bolt chat <peer-nickname>
bolt status
```

---

## Commands

```
bolt init                           Generate identity, start daemon
bolt id                             Show your fingerprint and randomart
bolt peers                          List online peers (LAN + relay)
bolt connect <ip>                   Manually add a peer by IP (VPN / internet)
bolt trust <peer> always|once|block Set per-peer trust level
bolt send <file> <peer> [peer2...]  Send a file to one or more peers
bolt send - <peer>                  Send from stdin (pipe support)
bolt chat <peer>                    Open 1:1 chat TUI
bolt chat --all                     Group chat with all trusted online peers
bolt daemon [--stealth]             Run daemon in foreground; --stealth = fully dark
bolt status                         Show daemon stats, peers, active transfers
bolt receive-dir <path>             Set receive directory for this session
```

---

## How identity works

When you run `bolt init`, an Ed25519 keypair is generated and stored at `~/.config/bolt/identity/`. This keypair is permanent — your **fingerprint** is the SHA-256 hash of your public key, formatted as colon-separated hex:

```
4b:4e:c9:4b:15:f4:38:30:4e:1c:73:75:f2:ee:0f:20:...
```

It also renders as a visual randomart (same algorithm as SSH `VisualHostKey`) so peers can verify your identity with a glance rather than comparing 64 hex characters.

```
+--[ bolt ]----+
|    .o       oo|
|   .o.      . o|
|   ..o...    .o|
|  E o *+.o   + |
|   + +..S o + +|
|    .  . . + * |
|          + O  |
|         . =   |
|          ...  |
+-[SHA256]------+
```

### TOFU — Trust On First Use

The first time two machines connect, bolt shows you the peer's fingerprint and randomart and asks you to confirm. **Verify the fingerprint out-of-band** — a phone call, Signal message, or in person. Once confirmed, the peer is saved to `~/.config/bolt/peers.toml` and future connections are automatic.

This is the same trust model SSH uses with `known_hosts`. If a fingerprint changes unexpectedly, bolt warns you loudly.

---

## Trust levels

Each peer has a trust level you control:

| Level          | Behaviour                                               |
| -------------- | ------------------------------------------------------- |
| `always-allow` | Files accepted automatically — no prompt                |
| `allow-once`   | Prompted for each incoming file (default for new peers) |
| `block`        | All connections silently rejected                       |

```bash
bolt trust alice-macbook always   # fire-and-forget
bolt trust work-server once       # prompt each time (default)
bolt trust stranger block         # never connect
```

---

## File transfer

Files are split into chunks and sent over parallel QUIC streams — one goroutine per chunk, up to 8 simultaneous streams per transfer. Each chunk is SHA-256 verified on arrival. The complete file is verified again before being written to the destination.

Transfers are **resumable** — if a connection drops mid-transfer, restarting picks up from the last verified chunk.

Chunk size is path-aware: large on LAN (4MB), smaller over relay (256KB).

```bash
# Send a file
bolt send video.mp4 alice-macbook

# Send to multiple peers simultaneously
bolt send report.pdf alice-macbook pi-homelab

# Pipe from stdin
tar czf - ./project | bolt send - alice-macbook
cat /var/log/syslog | bolt send - monitoring-server
```

Received files land in `~/Downloads` by default. Change it:

```toml
# ~/.config/bolt/config.toml
receive_dir = "/mnt/storage/incoming"
```

---

## Chat

```bash
# 1:1 chat — opens a TUI
bolt chat alice-macbook

# Group chat with everyone online
bolt chat --all
```

Chat is **ephemeral by default** — nothing is written to disk. To opt in to local logging:

```bash
bolt chat alice-macbook --log
```

Or set it permanently in config:

```toml
# ~/.config/bolt/config.toml
log_chat = true
log_transfers = true
```

Logs are written to `~/.local/share/bolt/logs/`.

---

## LAN vs internet

### Same network (LAN, VPN)

Peers discover each other automatically via mDNS. No configuration needed. Works on home networks, office LANs, conference WiFi, and any VPN that forwards multicast (Tailscale, WireGuard on same subnet).

For VPNs that block multicast, add peers manually:

```bash
bolt connect 10.8.0.5       # WireGuard VPN peer
bolt connect 100.64.0.3     # Tailscale peer
```

### Different networks (internet)

Requires a relay server (`boltd`) that you run on a VPS. The relay handles peer discovery and NAT hole-punching. File content is E2E encrypted before leaving your machine — the relay never sees plaintext.

```toml
# ~/.config/bolt/config.toml
relay = "https://relay.yourdomain.com"
relay_cert_fingerprint = "ab:cd:ef:..."   # only needed for self-signed certs
```

See [Relay setup](#relay-setup) below.

---

## Stealth mode

Running with `--stealth` makes your machine invisible to both mDNS and the relay. It will not appear in anyone's `bolt peers` list. You can still receive connections from peers who know your IP directly.

```bash
bolt daemon --stealth

# Connect to a stealth machine directly
bolt connect 192.168.1.42
```

---

## Relay setup

The relay (`boltd`) is a small Go binary you run on any VPS. It handles peer discovery and NAT hole-punching. It does **not** see file content or chat messages.

```bash
# Build the relay binary
go build -o boltd ./cmd/boltd

# Deploy to your VPS
scp boltd user@your-vps:~/
```

Configure it:

```toml
# boltd.toml
relay_secret = "your-long-random-secret-here"
port = 443
cert_dir = "/etc/boltd/certs"   # for Let's Encrypt
```

Install `coturn` for TURN fallback (handles ~25% of connections that fail NAT hole-punching):

```bash
apt install coturn
# configure /etc/turnserver.conf — see deploy/coturn.conf.example
```

Start as a service:

```bash
cp deploy/boltd.service /etc/systemd/system/
systemctl enable --now boltd
```

---

## Privacy

**What bolt never does:**
- No telemetry, no analytics, no call-home
- No auto-update that could be tampered with
- No anonymous connections — every peer has a verified identity
- No store-and-forward — messages are not held on any server
- No plaintext on the wire — TLS 1.3 over QUIC for all connections

**What the relay sees** (if you use internet mode):
- Which peer IDs are online and when
- IP addresses of connected peers

**What the relay never sees:**
- File content (E2E encrypted before leaving your machine)
- Chat messages (E2E encrypted)
- Who is talking to whom (peer-to-peer after relay-assisted handshake)

If you run your own relay on a VPS you control, you control all of this metadata.

---

## Configuration reference

```toml
# ~/.config/bolt/config.toml
version = 1

nickname = "alice-macbook"        # shown to peers
port = 7799                        # UDP port for QUIC
receive_dir = "~/Downloads"        # where incoming files land
log_chat = false                   # opt-in chat logging
log_transfers = false              # opt-in transfer history
stealth = false                    # suppress mDNS + relay registration

relay = ""                         # relay URL (leave empty for LAN-only)
relay_cert_fingerprint = ""        # pin relay cert (self-signed deployments)
max_relay_transfer_bytes = 5368709120   # 5GB cap for TURN-relayed transfers
```

---

## File layout

```
~/.config/bolt/
  identity/private.key    Ed25519 private key (0600 — never shared)
  identity/public.key     Ed25519 public key
  config.toml             User preferences
  peers.toml              Known peers and trust levels
  daemon.sock             Unix socket (runtime, auto-created)
  daemon.pid              Daemon PID (runtime, auto-created)

~/.local/share/bolt/
  logs/                   Opt-in chat and transfer logs
  incomplete/             In-progress transfer state (for resume)
```

---

## Build phases

| Phase              | Status   | What it adds                                         |
| ------------------ | -------- | ---------------------------------------------------- |
| 1 — Core transport | **Done** | Identity, QUIC, TOFU handshake, daemon, CLI skeleton |
| 2 — File transfer  | Planned  | Chunked parallel transfer, progress bar, resume      |
| 3 — Peer discovery | Planned  | mDNS auto-discovery, manual connect, daemon IPC      |
| 4 — Chat TUI       | Planned  | Real-time chat, bubbletea TUI, opt-in logging        |
| 5 — Internet path  | Planned  | Relay server, ICE hole-punch, TURN fallback          |
| 6 — Polish         | Planned  | Cross-compile releases, error message audit          |

---

## Architecture

See [architecture.md](architecture.md) for Mermaid diagrams covering:
- System overview
- Connection path selection (LAN vs internet)
- File transfer sequence
- TOFU identity verification flow
- Daemon auto-spawn logic

---

## Tech stack

| Concern       | Library                                            |
| ------------- | -------------------------------------------------- |
| Transport     | `quic-go` — QUIC over UDP, TLS 1.3 built-in        |
| Hole-punching | `pion/ice` — ICE candidate negotiation             |
| LAN discovery | `grandcat/zeroconf` — mDNS _bolt._udp              |
| Cryptography  | `golang.org/x/crypto` — Ed25519, ChaCha20-Poly1305 |
| TUI           | `charmbracelet/bubbletea` + `bubbles` + `lipgloss` |
| CLI           | `spf13/cobra`                                      |
| Config        | `BurntSushi/toml`                                  |
| Relay storage | `modernc.org/sqlite` — pure Go, no CGO             |
| TURN server   | `coturn` — deployed separately on VPS              |

---

## Contributing

This is an early-stage project. The architecture is intentionally kept simple and readable — code is written for humans first.

Before contributing, read the [plan](plan.md) to understand the design decisions and their rationale. Many decisions that look arbitrary have specific reasons documented there.

---

## License

MIT
