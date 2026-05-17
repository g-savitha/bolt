# Installing bolt

The **bolt** client is open source (MIT). LAN chat and file transfer are free.  
Internet relay (e.g. Hyd → Amsterdam) uses a **paid hosted service** — relay secrets and billing are not in this repo.

## Quick install (recommended)

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/g-savitha/bolt/main/scripts/install.sh | bash
```

Or a specific version:

```bash
./scripts/install.sh 0.1.0
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/g-savitha/bolt/main/scripts/install.ps1 | iex
```

Allow `bolt.exe` through the firewall for **private networks** and UDP **7799**.

---

## Homebrew (macOS / Linux)

**Option A — install script** (works before the formula SHA256 is updated):

```bash
curl -fsSL https://raw.githubusercontent.com/g-savitha/bolt/main/scripts/install.sh | bash
```

**Option B — tap** (after you create `g-savitha/homebrew-tap` and GoReleaser publishes the formula):

```bash
brew tap g-savitha/tap
brew install bolt
```

**Option C — formula from this repo** (update `sha256` in [packaging/homebrew/Formula/bolt.rb](packaging/homebrew/Formula/bolt.rb) from the release `SHA256SUMS` first):

```bash
brew install --formula https://raw.githubusercontent.com/g-savitha/bolt/main/packaging/homebrew/Formula/bolt.rb
```

---

## Scoop (Windows)

**Option A — install script:** see above.

**Option B — bucket** (after you create `g-savitha/scoop-bolt` or use the manifest below):

```powershell
scoop bucket add bolt https://github.com/g-savitha/scoop-bolt
scoop install bolt
```

**Option C — manifest from this repo** (update `hash` in [packaging/scoop/bolt.json](packaging/scoop/bolt.json) from the release first):

```powershell
scoop install path/to/packaging/scoop/bolt.json
```

---

## GitHub Releases (manual)

Download the archive for your OS/arch from [Releases](https://github.com/g-savitha/bolt/releases), verify `SHA256SUMS`, and put `bolt` on your `PATH`.

---

## Build from source

```bash
git clone https://github.com/g-savitha/bolt.git
cd bolt
go build -o bolt ./cmd/bolt
sudo mv bolt /usr/local/bin/
```

---

## Publish a release (maintainers)

1. Ensure tests pass: `go test ./...`
2. Tag: `git tag v0.1.0 && git push origin v0.1.0`
3. GitHub Actions runs GoReleaser and uploads assets + `SHA256SUMS`
4. Update `packaging/homebrew/Formula/bolt.rb` and `packaging/scoop/bolt.json` hashes from `SHA256SUMS`
5. Optional: create public repos `homebrew-tap` and `scoop-bolt` for `brew tap` / `scoop bucket`

Local snapshot (no publish):

```bash
goreleaser build --snapshot --clean
ls dist/
```

---

## Not supported

- **npm** / **pip** — wrong ecosystem for a Go CLI
- **MSI installers** — deferred; use Scoop or the install script

---

## Next steps

See [QUICKSTART.md](QUICKSTART.md) for `bolt init`, `bolt connect`, and `bolt chat`.
