# bolt — quick start

## Install

Pick one method. Full details: [INSTALL.md](INSTALL.md).

**macOS / Linux**

```bash
curl -fsSL https://raw.githubusercontent.com/g-savitha/flick/main/scripts/install.sh | bash
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/g-savitha/flick/main/scripts/install.ps1 | iex
```

**Homebrew** (after release SHA256 is set in the formula)

```bash
brew install --formula https://raw.githubusercontent.com/g-savitha/flick/main/packaging/homebrew/Formula/bolt.rb
```

**From source**

```bash
git clone https://github.com/g-savitha/flick.git
cd flick
go build -o bolt ./cmd/bolt
sudo mv bolt /usr/local/bin/
```

---

## On each machine

```bash
bolt init
bolt id          # share fingerprints once (text/call) to verify identity
```

Allow **UDP port 7799** through the firewall on private networks.

## Connect (same WiFi)

On machine A, find LAN IP:

- macOS: `ipconfig getifaddr en0`
- Linux: `hostname -I | awk '{print $1}'`
- Windows: `ipconfig` → WiFi IPv4

On machine B:

```bash
bolt connect 192.168.1.42    # A's IP
bolt status
```

## Chat

```bash
bolt chat <brother-nickname>
```

Type at `>`; `/quit` to exit. Nickname is from `bolt status` (usually hostname).

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `connect to bolt daemon` failed | `bolt init` or `bolt daemon` |
| `peer is not connected` | Other side runs `bolt connect <your-ip>` |
| No messages | Firewall UDP 7799; both daemons running |

## What's free vs paid

| Feature | Cost |
|---------|------|
| LAN chat (`bolt connect` + `bolt chat`) | Free |
| Internet relay (Hyd → Amsterdam, etc.) | Paid hosted service — `relay_token` in config (not in this repo) |

See [INSTALL.md](INSTALL.md) for packaging. Relay server (`boltd`) stays private / separate repo.
