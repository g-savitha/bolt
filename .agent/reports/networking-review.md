# Networking / Protocol Review — 2026-05-19

Reviewer: Networking & Protocols (read-only)
Scope: `plan.md`, `architecture.md`, `internal/transport/*`, `internal/proto/wire.go`, `internal/daemon/{daemon,connect}.go`, `internal/identity/identity.go`, `go.mod` (quic-go v0.59.1, Go 1.25.1).

---

## Executive summary

- The wire shape ships a **JSON `ChunkMsg` with a base64-encoded `Data` field**. That is the single biggest protocol-level mistake for Phase 2: base64 inflates the payload ~33%, forces a full unmarshal/alloc/copy per chunk, and caps a 4 MB chunk at ~5.6 MB on the wire — gigabit LAN goal becomes CPU-bound on the receiver before it becomes network-bound. Switch to a raw-bytes-after-JSON-header frame on dedicated streams before any sender/receiver code is written.
- **`quic.Config` is under-specified.** No `MaxIdleTimeout`, no `EnableDatagrams`, no `Allow0RTT`, no flow-control window tuning, no `Tracer`, and `KeepAlivePeriod=15s` is set against the quic-go default `MaxIdleTimeout=30s` — fine, but only by accident. The defaults inherited from quic-go v0.59.1 (CUBIC, default windows of ~512KiB initial / 6MiB max stream) will leave a single QUIC stream at well under 1 Gbps on a 1 ms RTT LAN; on a 150–250 ms intercontinental link they will leave a single connection at a few MB/s. This must be tuned before Phase 2.
- **Eight parallel streams is the wrong sender-side knob** when QUIC is already multiplexing on one congestion-controlled connection. On LAN, fan-out helps only because it parallelizes disk reads / SHA-256 / JSON encoding (the bottleneck is bolt's encoder, not the network). On TURN-relayed paths, 8 streams over one congestion window will actively *hurt* — flag this so the design does not lock it in.
- **TOFU plumbing has two real holes:** (a) `ClientTLSConfig.VerifyPeerCertificate` accepts every cert except `TrustBlock` — there is no "unknown peer" prompt, no first-seen pinning, no fingerprint-mismatch detection against `peers.toml`. (b) The server side does not run `VerifyPeerCertificate` at all (`ClientAuth: tls.RequireAnyClientCert` requires presence but not validity), so the server happily accepts any self-signed peer cert and only checks trust *after* TLS completes. Both are MITM-tolerant today.
- **Phase 5 plan ("ICE 75% / TURN 25%") is folklore.** Real measurements (Tailscale public reports, WebRTC stats from large deployments) put symmetric-NAT + CGNAT failure rates noticeably higher, especially on Indian mobile carriers (Jio/Airtel) which the Hyderabad↔Amsterdam example will hit. Plan should drop the numbers or footnote them as unverified. Also: wrapping pion/ice's per-peer `net.Conn` into a quic-go `Transport` is the riskiest piece of plumbing in the project and is not described concretely in the plan.

---

## Critique by area

### 1. QUIC configuration

**Findings**
- `internal/transport/quic.go`:
  - `KeepAlivePeriod: 15s` — sets quic-go's PING interval.
  - `MaxIncomingStreams: 1000`, `MaxIncomingUniStreams: 1000`.
  - `HandshakeIdleTimeout: 10s` — fine.
  - **Not set:** `MaxIdleTimeout`, `EnableDatagrams`, `Allow0RTT`, `DisablePathMTUDiscovery`, `InitialPacketSize`, `InitialStreamReceiveWindow` / `MaxStreamReceiveWindow`, `InitialConnectionReceiveWindow` / `MaxConnectionReceiveWindow`, `Tracer`.
- quic-go v0.59.1 defaults: `MaxIdleTimeout=30s`, datagrams disabled, 0-RTT disabled, PMTUD enabled, CUBIC, initial stream/conn windows 512 KiB / 512 KiB, max stream/conn windows 6 MiB / 15 MiB.

**Risk**
- 15 s keep-alive is *less than* the 30 s default `MaxIdleTimeout` — works today, but if anyone later sets `MaxIdleTimeout` lower than 2× keep-alive the connection will silently die. Per RFC 9000 §10.1 keep-alive must be sufficiently smaller than the idle timeout *and* below the lowest expected NAT mapping timeout (≥30 s for most home NATs, 30–60 s for UDP on enterprise NATs).
- Default 6 MiB max stream window × single stream × 200 ms RTT caps the per-stream throughput at ~30 MB/s ≈ 240 Mbps. The plan's "wire speed gigabit" is unreachable on a single stream without window tuning. Eight streams paper over this but only at the cost of more sender-side CPU.
- CUBIC over a low-loss LAN is fine. CUBIC over a high-RTT, high-loss TURN-relayed leg between Hyderabad and Amsterdam will under-utilise the link. quic-go v0.59.1 **does not expose BBR/BBRv2** as a public option (the internal CC code carries a CUBIC implementation and an experimental Reno; no BBR is wired into `quic.Config`). Document this as a known limitation, not a tunable.
- PMTUD is on by default in quic-go and probes upward from `InitialPacketSize` (1252 default). Useful, but not free — every ~minute it can briefly probe higher and bounce back if blackholed. Mention in operator docs that aggressive UDP fragmentation filtering on intermediate firewalls can permanently pin packets at 1200 B.
- Anti-amplification / address validation on the server side is handled by quic-go automatically (3× anti-amplification cap, optional retry tokens via `Transport.MaxUnvalidatedHandshakes`). For a daemon that may face the open internet (Phase 5), set `Transport.MaxUnvalidatedHandshakes` and provide a `Transport.VerifySourceAddress` callback. RFC 9000 §8.

**Recommendation**
- Set `MaxIdleTimeout = 60s` explicitly; reduce `KeepAlivePeriod` to `20s` so it is < `½ × MaxIdleTimeout` and < typical NAT timeout. (RFC 9000 §10.1)
- Set initial+max stream/connection windows up front:
  - LAN-tuned defaults: `InitialStreamReceiveWindow = 4 MiB`, `MaxStreamReceiveWindow = 64 MiB`, `InitialConnectionReceiveWindow = 8 MiB`, `MaxConnectionReceiveWindow = 256 MiB`. Lets a single stream actually fill a 1 Gbps × 1 ms BDP.
  - Memory cost: bounded per connection; pre-Phase 2 the daemon has at most a handful of peers, so 256 MiB ceiling per peer is acceptable.
- `EnableDatagrams: true` on both ends from day one. Cost is zero if unused; flips the bit in the QUIC transport-parameters so the receiver advertises support (RFC 9221 §3). If decided later the plan can use datagrams for chat presence/typing/heartbeat without a wire-version bump.
- `Allow0RTT: false` — lock this in writing. 0-RTT replay risk is real for any state-changing command (file send), Phase 1 chat is also stateful. Revisit only when a designed `Replay-Safe-Only` carve-out exists. RFC 9001 §5.6, RFC 8470.
- BBR: document that quic-go v0.59.1 ships only CUBIC. If LAN gigabit fills are missed by >20%, the tuning lever is window sizes and packet pacing (`DisablePathMTUDiscovery=false`, larger initial packet size), not CC algorithm.
- Set a default `Tracer` (`logging.NewMultiplexedTracer`) gated by `BOLT_QLOG=1` so we can capture qlog from real users for Phase 5 debugging.

**Severity**
- P0: missing `MaxIdleTimeout` and unbounded windows.
- P0: explicit `Allow0RTT: false` and ALPN decision (covered in §5).
- P1: enable datagrams now for forward compatibility.
- P2: BBR, qlog tracer.

---

### 2. TLS / TOFU correctness

**Findings**
- `identity.TLSCertificate()` embeds the Ed25519 public key bytes in `SubjectKeyId` and signs the cert with a freshly-generated ECDSA P-256 key. Code comment acknowledges this is a "pragmatic workaround". `ExtractPublicKeyFromCert` reads the public key back out of `SubjectKeyId`. This **does** bind the Ed25519 key to the cert at construction time on this peer, but **does not cryptographically bind** the Ed25519 key to the cert's signature — anyone can mint a self-signed cert with any 32 bytes in `SubjectKeyId`. Today the TOFU check uses only those bytes; an attacker with a different ECDSA wrapper key but a copy of the target's Ed25519 public key would still produce a cert that fingerprints the same.
- This is partially mitigated by the application-level handshake in `PeerConn.Handshake`: `validateHandshake` compares `Fingerprint(handshake.PublicKey)` against `pc.peerFingerprint` (extracted from `SubjectKeyId`). They will always match because the attacker controls both. **It does NOT prove the peer holds the Ed25519 private key** — there is no signature challenge over a TLS-handshake-derived nonce.
- `ClientTLSConfig` has `InsecureSkipVerify: true` and a custom `VerifyPeerCertificate`. Looks correct in principle.
- `ServerTLSConfig` uses `ClientAuth: tls.RequireAnyClientCert` but does **not** set `VerifyPeerCertificate`. The server therefore accepts any client cert with any `SubjectKeyId` (any 32 bytes). The blocked-fingerprint check runs after `conn.Accept()` in `authenticateIncoming`. This means: an attacker can establish a TLS+QUIC connection to a bolt daemon using any forged identity; the daemon only finds out post-handshake.
- `peerVerifier()` in `daemon/connect.go` returns `(true, nil)` for every unknown peer. There is no "first-seen" pinning, no comparison against `peers.toml`, no IPC prompt to the user. The plan describes a TOFU prompt; the code does not implement it.
- `tls.Config.SessionTicketsDisabled` and `ClientSessionCache` are not set. quic-go's TLS 1.3 path can issue session tickets; in v0.59.1, session tickets are also the mechanism by which 0-RTT is enabled if `Allow0RTT` is on. Since 0-RTT is currently off, session tickets are functionally inert but should still be turned off to prevent future re-enablement leaking session state to disk (the default `tls.ClientSessionCache` is in-memory only, so today this is theoretical).

**Risk**
- **MITM with forged identity is undetected at TLS time.** A malicious LAN peer can claim any fingerprint and finish the QUIC handshake. The application-level cross-check (`validateHandshake`) does not catch this because the attacker also crafted the handshake message. The server only rejects *blocked* peers.
- **Cert↔key binding is by construction, not by signature.** Without a proof-of-possession of the Ed25519 private key, the TOFU model is weaker than SSH's: SSH-known-hosts pins the SSH server's host key, and the server proves it by signing the SSH transport-layer key exchange. Here, the Ed25519 key never signs anything in the live handshake.
- **No "unknown peer prompt" today.** The plan claims one in Phase 1 verification; the code accepts everyone. This is also a UX bug (peers join silently), but more importantly a security bug because the TOFU model relies on the user actively pinning the first-seen fingerprint.

**Recommendation**
- Add a signed-nonce proof-of-possession to `HandshakeMsg`:
  - After TLS completes, derive a 32-byte challenge from `tls.ConnectionState.TLSUnique()` (or, on TLS 1.3 where `tls_unique` is empty, use `ExportKeyingMaterial("bolt-handshake", nil, 32)` — RFC 5705).
  - Each side signs that challenge with its Ed25519 private key and includes the signature in `HandshakeMsg`.
  - The peer verifies the signature against the public key extracted from `SubjectKeyId`. Mismatch → reject. This closes the cert/key binding hole *without* changing the cert generation code.
- Wire a real TOFU verifier:
  - `peerVerifier()` should: (i) reject blocked, (ii) if known, require *exact* fingerprint match (already true via TLS extract, but make it explicit); (iii) if unknown, push `tofu_prompt` IPC event, block up to 30 s, accept if user confirms, then persist to `peers.toml`.
  - Server-side: install the same verifier as `tls.Config.VerifyPeerCertificate` on `ServerTLSConfig`. Today, the post-handshake check in `authenticateIncoming` lets an attacker waste CPU on a full QUIC handshake before being rejected. Failing in TLS is cheaper and more idiomatic.
- Explicit defensive flags:
  - `tls.Config.SessionTicketsDisabled = true` until 0-RTT is reconsidered.
  - `tls.Config.ClientAuth: tls.RequireAndVerifyClientCert` is *not* what we want (it needs a CA pool); keep `RequireAnyClientCert` but add `VerifyPeerCertificate` server-side.
- Document the cert validity choice (100 years, `KeyUsageDigitalSignature`, `ExtKeyUsage` server+client auth) as intentional — looks fine.

**Severity**
- P0: missing server-side `VerifyPeerCertificate` + TOFU prompt + per-peer pinning.
- P0: missing Ed25519 proof-of-possession.
- P1: explicit `SessionTicketsDisabled`.

---

### 3. Stream model and framing

**Findings**
- `proto.WriteStreamType` / `ReadStreamType` write a single byte at the start of each QUIC stream. Following bytes are length-prefixed (`uint32` big-endian) JSON, capped at 8 MiB (`maxFrameSize`).
- `ChunkMsg.Data []byte` — `encoding/json` serializes `[]byte` as base64 standard-encoded string. A 4 MiB chunk becomes ~5.6 MiB of JSON, plus the header and quotes. **The 8 MiB cap is dangerously close** — there is no slack for the JSON envelope around large chunks; bump or rethink.
- One stream type per QUIC stream is good. Mixing chunk data and control messages on the same stream would be a footgun (HoL blocking is per-stream in QUIC; RFC 9000 §2).

**Risk**
- Base64 in chunks: ~33% wire overhead, full allocation of a base64 string + a `[]byte` decode buffer per chunk, and `json.Unmarshal` of a multi-megabyte payload — that is, GC pressure and an O(N) memcpy on every chunk in both directions. CPU will cap throughput well before the NIC.
- 8 MiB envelope cap: a 4 MiB binary chunk fits, but a 5 MiB chunk does not. The plan also reserves "TURN: 256 KiB" — fine. But the cap forces sender and receiver to agree on the same chunk-size policy, otherwise a sender that picks 6 MiB (e.g., a future tuning experiment) cannot send a single ChunkMsg at all.
- JSON for non-bulk frames (FileHeader, TransferAck, TransferDone, ChatMsg, HandshakeMsg) is fine: human-debuggable, evolvable. Keep it for control.

**Recommendation**
- **Pick one framing for chunk payloads** before Phase 2:
  - **Option A (preferred): header-then-raw.** On a chunk-carrying stream, write `StreamFile` byte, then a single length-prefixed JSON `ChunkHeader{v, transfer_id, index, chunk_hash, len}`, then exactly `len` raw bytes (no JSON, no base64). End-of-stream from the sender signals the chunk is complete. The receiver hashes streaming.
  - **Option B: one-chunk-per-stream, no per-message envelope.** The transfer's *control* stream announces chunk size and ordering via FileHeader; each chunk stream carries `[stream_type=0x03][8-byte chunk_id (transfer_id_hash<<32 | index)][raw bytes]`. Most compact but tightens coupling.
- Either way, raise `maxFrameSize` to 16 MiB for JSON envelopes only and *remove* `[]byte` from `ChunkMsg` (it should not exist after the framing decision).
- Document: per-stream HoL blocking does not cross streams (RFC 9000 §2.1). This is why each chunk goes on its own stream and why the chat stream must never be reused for bulk data.

**Severity**
- P0: encoding decision must be made before sender/receiver code is written.
- P1: bump or document `maxFrameSize`.

---

### 4. File transfer chunking strategy

**Findings**
- Plan: `min(8, totalChunks)` parallel streams, 4 MiB chunks on LAN, 256 KiB chunks over TURN.
- Per-chunk SHA-256 + whole-file SHA-256: both computed.
- Resume checkpointed every 10 chunks → up to 40 MiB redo on crash with LAN chunk size, up to 2.5 MiB redo on TURN. Asymmetric and not in proportion to chunk size.
- Receiver uses `ftruncate` + `WriteAt`: standard parallel-write recipe.

**Risk**
- **8 streams is a sender-side parallelism number, not a network-side one.** QUIC already congestion-controls the entire connection; adding more streams cannot exceed the connection's congestion window. On LAN at 1 Gbps the benefit comes from: (i) parallel reads against the source file, (ii) parallel SHA-256 hashing, (iii) parallel JSON encoding. All three are CPU/disk-side. On TURN-relayed paths, more streams *cannot* speed up a single bottleneck and slightly hurt by increasing per-packet overhead and head-of-line interactions inside the TURN allocation. RFC 8656 TURN data channels add their own framing overhead per packet, magnifying the cost of small chunks.
- 4 MiB chunks on LAN are good for throughput but increase tail-latency for resume: the last 4 MiB is the most likely to be wasted. Acceptable trade.
- 256 KiB chunks over TURN are *too small* if every chunk pays a JSON envelope cost. With raw-bytes framing they are reasonable; with base64-JSON they collapse to mostly overhead.
- "Checkpoint every 10 chunks" is decoupled from chunk size — should be "every 32 MiB of acknowledged data" or similar.
- `WriteAt` correctness:
  - On Linux ext4/xfs: concurrent `pwrite` to disjoint byte ranges is safe; sparse files behave correctly; `fallocate` is preferable to `ftruncate` for performance (avoids zero-fill on first write for ext4 in some configurations). On xfs, `fallocate` with `FALLOC_FL_KEEP_SIZE` can be used.
  - On macOS APFS: `truncate(2)` creates a sparse file; APFS does no preallocation. Concurrent `pwrite` is safe to disjoint ranges. No `fallocate`; use `F_PREALLOCATE` via `fcntl` for hint-only preallocation if desired.
  - `fsync` must be called at the end before the rename; without it, a crash after rename can leave a logically-named but partially-flushed file (rename → metadata flush before data flush on ext4 with `data=ordered` is the usual default but not guaranteed for all FSes). Plan should call this out.
- Path-aware chunk size: the path type is unknown until ICE completes in Phase 5. For Phase 1/2 we are always LAN. **Today**, `FileHeader.ChunkSize` is sender-chosen and the receiver agrees. Forward-compat is fine because the field already exists. Default to `LAN=4MiB` until `PeerConn.PathType` is wired in Phase 5.

**Recommendation**
- Add `PeerConn.PathType` (`PathLAN`, `PathDirect`, `PathTURN`) defaulting to `PathLAN`. Phase 5 sets it after ICE. Sender uses it to pick chunk size at `FileHeader` build time.
- Make N (parallel streams) a function of `PathType`:
  - `PathLAN`: 8 (good for disk/CPU parallelism).
  - `PathDirect` (ICE direct over internet): 4.
  - `PathTURN`: **1**. One chunk at a time over a single stream is correct over a single 5-tuple bottleneck. Document.
- Reframe "checkpoint every 10 chunks" as **"checkpoint every 32 MiB of data acknowledged"** so it adapts to chunk size.
- Receiver: use `fallocate` on Linux when available; otherwise `ftruncate`. Call `fsync(fd)` (or `fdatasync`) before atomic rename. Call out the macOS no-op for fallocate.
- Per-chunk SHA-256 is a defense against a malicious-or-broken sender mid-stream. Whole-file SHA-256 is the receipt. Keep both; cost is one extra hash pass that streams in-line with the write. Total CPU cost ~1.5 GB/s on modern hardware, not a bottleneck.

**Severity**
- P0: parallel-stream count must be path-aware before Phase 2 finalises sender code.
- P0: chunk encoding (covered in §3).
- P1: resume granularity should be data-size based, not chunk-count based.
- P1: receiver fsync + fallocate notes.

---

### 5. Wire protocol versioning

**Findings**
- ALPN: `bolt/1` is set in `tls.go`. Good — peers with mismatching ALPN fail TLS, no application-layer error needed.
- Per-message `"v":1` in JSON. Daemon does not currently inspect `v` consistently (`validateHandshake` checks `msg.V != proto.WireVersion`; other receivers do not). Some duplication.
- Unknown stream type handling: `daemon.routeStream` falls through to `stream.Close()` and prints a warning. No `CloseWithError` code, no peer signaling.

**Risk**
- A peer running a future minor revision that adds a new stream type byte will see its stream closed silently. Recovery is undefined. RFC 9000 §4.6 (stream resets) is the right primitive.
- The "ALPN gates major, per-message v gates minor" pattern is the right one. Keep it.

**Recommendation**
- Lock ALPN as `bolt/1` for the lifetime of wire-version 1. Bumping a breaking change ⇒ `bolt/2`.
- On unknown stream type: send `quic.Stream.CancelRead(0x10)` and `CancelWrite(0x10)` with an application-level error code dedicated to "unknown stream type" (e.g., 0x1000), defined as a constant in `proto`. Peer can log and reconnect with a downgraded protocol if it wants.
- Centralise version check in a single `proto.CheckVersion(v int) error` helper and call it in every `ReadFrame` site.

**Severity**
- P1: unknown-stream-type behaviour.
- P2: version-check centralisation.

---

### 6. IPC / daemon's network side

**Findings**
- Daemon listens on `:7799` UDP. Configurable via `cfg.Port`. Not explicitly documented as a firewall requirement.
- Loopback peers (two daemons on the same host) should work: quic-go binds via `quic.ListenAddr(":7799", ...)`. Two daemons on the same host would need distinct ports. Useful for integration tests.

**Risk**
- A user behind a host firewall (UFW, macOS Application Firewall) will see silent inbound failures. No diagnostic in `bolt status`.
- UDP-on-loopback in macOS works but `127.0.0.1` and `::1` are separate sockets — bind to `0.0.0.0:7799` and `[::]:7799` (or use `udp` network with empty host) to cover both stacks. Today `:7799` resolves IPv4+IPv6 dual-stack via Go's net stack on Linux but only IPv6-only on some macOS configurations (`net.IPv6unspecified` default). Test on both.

**Recommendation**
- `bolt status` should display "Listening on UDP:7799 (IPv4 [✓/✗], IPv6 [✓/✗])" after probing locally.
- Document the firewall requirement in README: "open inbound UDP/7799" for LAN use; for internet use, the relay-mediated path uses random outbound UDP and does not require port forwarding.
- For loopback tests, expose a `--port` flag on `bolt daemon` (already partly in `cfg.Port`) and a `--config-dir` flag for parallel daemons.

**Severity**
- P2.

---

### 7. NAT traversal plan (Phase 5)

**Findings**
- pion/ice + coturn + custom `/punch` is the right shape.
- "ICE 75% / TURN 25%" success number is unsupported by any citation in the plan.
- Plan does not specify ICE-lite vs full-ICE, IPv4/IPv6 candidate gathering order, or simultaneous-open vs prearranged-controlling-role.
- quic-go integration: plan says "wrap established UDP socket as `net.PacketConn` → pass to `quic-go` `Transport` API (not `DialAddr`)". The actual quic-go v0.59.1 surface is:
  - `quic.Transport` struct (fields: `Conn net.PacketConn`, `MaxUnvalidatedHandshakes`, `VerifySourceAddress`, etc.).
  - `Transport.Dial(ctx, *net.UDPAddr, *tls.Config, *quic.Config) (*quic.Conn, error)`.
  - `Transport.Listen(*tls.Config, *quic.Config) (*quic.Listener, error)`.
  - `Transport.DialEarly` for 0-RTT (off in v1).
- There is **no** `Transport.NewConnection`. The naming the plan uses is wrong.

**Risk**
- pion/ice gives you an `*ice.Conn` (`net.Conn`, not `net.PacketConn`). To hand it to `quic.Transport.Conn`, you need a `net.PacketConn` adapter:
  - `ReadFrom` returns the bytes from the single ICE peer with that peer's remote addr.
  - `WriteTo(buf, addr)` ignores `addr` and writes to the ICE peer.
  - This adapter is straightforward (~50 LOC) but it must reject packets from any other source (pion already does this) and must thread `SetDeadline` correctly.
- **One ICE session = one 5-tuple = one QUIC connection.** You cannot run a quic-go `Listener` on top of an ICE conn and expect inbound connections from multiple peers. The server-side daemon has two distinct UDP listeners after Phase 5:
  - `:7799` plain UDP, behind the QUIC listener (for LAN + manually-connected peers).
  - One quic-go `Transport` *per* ICE-negotiated peer, each backed by its own pion `ice.Conn`.
- Simultaneous-open: ICE will yield a working bidirectional UDP pair; one side must `Dial` and the other must `Listen` via `Transport`. The relay's `/punch` handshake must pre-assign controlling vs controlled (ICE Lite role) — coordinate this so both sides do not try to `Dial`. Use peer fingerprint lexicographic ordering as a tiebreaker.
- IPv4/IPv6 dual stack: pion/ice gathers both. quic-go does not care, but the wrapping `PacketConn` must report the correct local address for QUIC's path validation. Test this end-to-end before believing the design.
- CGNAT: very high in India mobile (Jio/Airtel use CGNAT extensively; estimates of 30–60% of mobile traffic). The Hyderabad↔Amsterdam example will hit it. Symmetric-NAT on both sides defeats vanilla ICE; that traffic goes to TURN. Plan should size TURN bandwidth on this expectation, not on a 25% number.
- QUIC connection migration: when a phone or laptop changes IP (Wi-Fi → cellular, sleep/wake), QUIC's connection migration (RFC 9000 §9) allows the client to keep the same connection over a new 4-tuple. quic-go v0.59.1 supports client-initiated migration on the server side (the server accepts the new address and probes). **However**, when QUIC is sitting on top of an ICE-managed `PacketConn`, migration breaks the abstraction — the local "address" did not change from quic-go's perspective even though the underlying ICE pair changed. Phase 5 needs an explicit story: either rebuild ICE on every network-change event and re-run the bolt application handshake (simple, drops the connection), or wire an ICE re-nomination event into quic-go's path validation (complex). Plan should document this is *out of scope for Phase 5* and document handover as "reconnect after 5 s".

**Recommendation**
- Strike the "75% / 25%" line. Replace with: "ICE success rate is network-dependent; deployments behind symmetric NAT or CGNAT will fall back to TURN. Operator should size TURN bandwidth assuming 30–50% of internet-leg traffic flows through it." Cite RFC 8445 §2.
- Phase 5 design notes:
  - Use **full ICE** on both peers (we control both). ICE-lite is for server-only deployments.
  - Role assignment: lexicographically smaller peer fingerprint = controlling. Avoid coin flips.
  - Build the `net.PacketConn` adapter as an `internal/transport/icepacketconn.go` with explicit tests using a `pipe`-like pair before the real ICE integration.
  - One `quic.Transport` per peer, both sides. Server side calls `transport.Listen`; client side calls `transport.Dial`. The "server" is whoever was controlling at ICE time.
  - Handover: on `network change` events (macOS / Linux netlink), close all peer connections and reconnect — do not attempt in-place QUIC migration over ICE in v1.
- TURN allocation lifetime per RFC 8656: default 10 min, refreshed via Refresh request. coturn handles this; the bolt daemon must keep its TURN-allocated app socket alive by sending application traffic at least every 5 min. If idle longer, do a no-op QUIC PING (driven by `KeepAlivePeriod`).

**Severity**
- P0 for the Phase 5 risk budget: pion/ice → quic.Transport plumbing is the project's hardest piece and has no prototype.
- P1: ICE-lite vs full-ICE decision.
- P1: TURN bandwidth sizing.

---

### 8. MTU / PMTUD

**Findings**
- Plan does not mention MTU.
- quic-go v0.59.1 defaults: `InitialPacketSize = 1252` (IPv4-safe), PMTUD enabled, probing up to ~1452 typically.
- QUIC initial packets are capped at 1200 B (RFC 9000 §14.1) regardless of PMTUD — anti-amplification floor.

**Risk**
- A jumbo-frame LAN (9000 MTU) will not be exploited; quic-go does not probe that high by default. Acceptable for v1.
- A tunnel with a smaller MTU (Wireguard with default 1420 + bolt over QUIC = 1420 − 60 − 16 ≈ 1344 effective) will be discovered by PMTUD but throughput will dip during probing.
- Datagrams (RFC 9221) have a smaller max payload than streams: roughly `connection MTU − overhead` per datagram, no fragmentation. For 1200 B floor that is ~1180 B per chat presence packet. Fine for chat, must not be used for chunks.

**Recommendation**
- Document the cap in `proto` package doc: "chat datagrams (when enabled) must fit in a single QUIC datagram, ≤ 1100 B payload to be safe across PMTUD probing".
- Leave `InitialPacketSize` at the default; do not raise it pre-Phase 5.

**Severity**
- P2.

---

### 9. Connection migration

**Findings**
- Not exercised. No test plan.

**Risk**
- Laptop sleep → wake: QUIC client *should* migrate seamlessly when the 4-tuple changes. If `KeepAlivePeriod` PING arrives within `MaxIdleTimeout` of the new path being usable, the connection survives. If sleep > idle timeout, connection is gone — daemon must reconnect.
- Mobile (Phase 5+): handover from Wi-Fi to cellular. As noted in §7, this interacts badly with ICE.

**Recommendation**
- Phase 1 (today): set `MaxIdleTimeout=60s`, `KeepAlivePeriod=20s`. Document in README that closing the laptop lid for > 60 s drops peer connections; daemon will reconnect on next outbound action or on mDNS rediscovery.
- Phase 2 integration test: bind the QUIC client to `127.0.0.1:RAND1`, perform a transfer, bind a second socket to `127.0.0.1:RAND2`, force a path migration via `Conn.MigrateUDPSocket` (if exposed — in v0.59.1 the public surface is `Transport.MigrateUDPSocket`; otherwise rebind the underlying `PacketConn`). Verify the transfer completes.

**Severity**
- P2 for Phase 2; P0 to budget into Phase 5.

---

### 10. Relay (Phase 5) protocol

**Findings**
- HTTPS endpoints for `/register`, `/heartbeat`, `/lookup`, `/punch` — control plane, fine.
- HMAC token: `HMAC-SHA256(relay_secret, peer_fingerprint)`.
- Heartbeat every 30 s, marked offline after 90 s.
- coturn for TURN.

**Risk**
- **Single relay_secret as HMAC key.** If the relay binary or its config leaks, every peer's token is derivable forever. There is no rotation strategy and no per-peer salt.
- **HMAC over fingerprint with no expiry** means tokens are non-revocable except via the fingerprint blocklist. A revoked-then-re-trusted peer would have to be re-issued the same deterministic token — fine, but a leaked token cannot be invalidated without rotating the global secret and forcing every peer to re-handshake.
- 30 s heartbeat vs typical UDP NAT mapping timeout: per RFC 4787 §4.3, the recommended minimum NAT UDP timeout is 2 min, but real-world home routers (especially small TP-Link / D-Link kits, Indian ISP routers) idle UDP mappings out at **30–60 s**. A 30 s heartbeat with jitter and a slow heartbeat arrival can race with mapping expiry. 25 s is safer.
- TURN allocation refresh: coturn defaults to 600 s. Auto-handled. But the **client-side bolt daemon** must keep traffic flowing through its TURN allocation, otherwise the relay tears down the allocation between refreshes if idle. quic-go's `KeepAlivePeriod` PING handles this.

**Recommendation**
- HKDF the per-peer token:
  - `info = "bolt-relay-token-v1" || peer_fingerprint || epoch_id`.
  - `token = HKDF-SHA256(relay_secret, salt=relay_id, info=info, len=32)`.
  - Rotate `epoch_id` monthly; relay accepts the current and previous epoch's tokens during a 24 h overlap window. Now a leaked relay_secret triggers epoch rotation and old tokens become invalid within 24 h.
  - Cite RFC 5869 for HKDF, RFC 2104 for HMAC.
- Heartbeat: **25 s** with ±5 s jitter, marked offline after 75 s.
- Document: TURN allocation lifetime (RFC 8656 §6), keep-alive responsibility on the client.
- Per-IP token bucket (10 req/s burst 100) is fine, but apply also to `/punch` — that endpoint is the most attractive to abuse for cross-protocol forging.

**Severity**
- P1: HKDF token derivation and rotation.
- P1: heartbeat interval tweak.
- P2: docs.

---

## Decisions the PO must lock BEFORE Phase 2 starts

These are designed to be answered in one sit-down. Each is a binary or short multi-choice with my recommendation. Phase 2 sender/receiver code cannot start until they are decided.

1. **ChunkMsg payload encoding.** JSON-base64 inside one frame *vs* JSON header + raw bytes on the rest of the stream.
   - **Recommend:** header-then-raw. Removes 33% wire overhead and a full unmarshal/copy per chunk. Eliminates the 8 MiB envelope cap risk.
2. **One-chunk-per-stream *vs* N-streams-round-robin.**
   - **Recommend:** one-chunk-per-stream on Phase 2 paths (LAN, ICE-direct). Simpler receiver state, natural backpressure via stream-level flow control. Revisit only if profiling shows OpenStream/CloseStream cost dominates (~1% on LAN at 4 MiB chunks — it won't).
3. **Parallel stream count by path type.**
   - **Recommend:** LAN=8, ICE-direct=4, TURN=1. Add `PeerConn.PathType` field today with default `PathLAN`.
4. **QUIC datagrams: enable in `quic.Config` now.**
   - **Recommend:** yes. Zero cost if unused; forward-compatible for chat presence / typing / heartbeat in Phase 4 without a wire-version bump.
5. **0-RTT posture: explicitly disabled in v1.**
   - **Recommend:** `Allow0RTT: false`, `SessionTicketsDisabled: true`. Document.
6. **ALPN string: `bolt/1`** for the duration of wire-version 1.
   - **Recommend:** lock. Bumping to `bolt/2` is the only way to ship a breaking wire change.
7. **Per-stream and connection idle timeouts.**
   - **Recommend:** `MaxIdleTimeout=60s`, `KeepAlivePeriod=20s`, `HandshakeIdleTimeout=10s` (already set). Document NAT mapping expectations.
8. **TLS-binding decision.** Add Ed25519 proof-of-possession (signature over RFC 5705 exported keying material) to `HandshakeMsg`.
   - **Recommend:** yes, before Phase 2 — the TOFU model is weaker than advertised without it.
9. **Default UDP port and IP stack.** `:7799` UDP; bind to both IPv4 and IPv6 on the daemon.
   - **Recommend:** confirm dual-stack on Linux and macOS; document inbound UDP firewall requirement.
10. **Receiver windows and chunk-size policy.** Initial 4 MiB / max 64 MiB per stream; chunk size 4 MiB on LAN, 256 KiB on TURN.
    - **Recommend:** wire receiver windows into `quic.Config` now (cost: a few hundred KiB of buffer); chunk size remains in `FileHeader` and is sender-chosen.

---

## Phase 2 networking prerequisites

Sized at 1–3 engineer-days each, ready to become backlog stories.

1. **Tighten `quic.Config` defaults (1 day).** Set `MaxIdleTimeout`, `KeepAlivePeriod`, `EnableDatagrams: true`, `Allow0RTT: false`, `InitialStreamReceiveWindow`, `MaxStreamReceiveWindow`, `InitialConnectionReceiveWindow`, `MaxConnectionReceiveWindow`. One Go file change, one test that asserts the values, one README line. Acceptance: `go test ./internal/transport/...` plus a manual `iperf3`-style throughput test on loopback showing > 700 Mbps on a single stream.

2. **Wire real TOFU verifier into `ClientTLSConfig` and `ServerTLSConfig` (2 days).** Add `peerVerifier` that:
   - Looks up `peers.toml`.
   - Rejects blocked, accepts always-allow.
   - For unknown: publishes an IPC `tofu_prompt` event; blocks up to 30 s; persists on accept.
   - Install on both client and server `tls.Config`.
   Acceptance: integration test in `internal/daemon` with two in-process daemons; first contact triggers prompt, second contact does not, blocked peer is rejected at TLS time. Closes the silent-acceptance hole.

3. **Add Ed25519 proof-of-possession to `HandshakeMsg` (1 day).** Sign `ExportKeyingMaterial("bolt-handshake", nil, 32)` (RFC 5705) with the local Ed25519 key; peer verifies against `SubjectKeyId`. Update `wire_test.go` round-trip test plus a new test that forges a cert with a stolen public key and asserts the handshake fails.

4. **Decide and implement chunk framing (2 days).** Implement Option A (recommended): `proto.ChunkHeader{v, transfer_id, index, chunk_hash, len}` length-prefixed JSON, followed by exactly `len` raw bytes on a dedicated `StreamFile` chunk-stream. Remove `ChunkMsg.Data []byte`. Update wire_test.go; add a 4 MiB chunk round-trip test that asserts no base64 encoding occurs.

5. **Stream type router in `daemon.routeStream` (1 day).** Register dispatch for `StreamFile` to a handler bound to the connection context (`pc.runCtx`). Unknown stream types: send `CancelRead/CancelWrite` with application error code `0x1000` (proto.ErrUnknownStreamType). Add a unit test that an unknown 0xFF stream is rejected without leaking the goroutine.

6. **`PeerConn.OpenStream` already exists; add `PeerConn.OpenChunkStream(ctx, transferID) (*quic.Stream, error)` (0.5 days).** Convenience helper that opens a `StreamFile` stream, writes a chunk header, and returns the stream ready for the caller to write raw bytes into. Mirror `AcceptChunkStream` on the receiver side.

7. **Add `PeerConn.PathType` field with default `PathLAN` (0.5 days).** Field, getter, setter (used by Phase 5). Sender consults this when building `FileHeader.ChunkSize` and when choosing parallelism.

8. **Receiver backpressure: bounded chunk-buffer pool (1 day).** A `sync.Pool` of `[chunkSize]byte` slices, capped at `2 × N_parallel_streams` outstanding allocations per transfer (configurable; default 16 for LAN). When the pool is empty, the accept loop blocks rather than reading more chunks — backpressure flows naturally through QUIC stream-level flow control.

9. **Stream read/write deadline policy (1 day).** Wrap every chunk read/write in `stream.SetReadDeadline(time.Now().Add(30s))` and reset after each successful chunk. Context cancellation closes the stream via `CancelRead/CancelWrite`. Without this, a stalled peer hangs a transfer goroutine indefinitely.

10. **Define and freeze `FileHeader`, `TransferAck`, `ChunkHeader`, `TransferDone` schemas with `v=1` (1 day, parallel to #4).** Owner: networking. Output: one PR updating `internal/proto/wire.go` and `wire_test.go` with golden round-trip tests for each shape. After this lands, sender/receiver code can start in parallel.

11. **(Optional, 1 day) Add a qlog `Tracer` gated by `BOLT_QLOG=1`.** Writes `~/.local/share/bolt/qlog/<peer>-<connID>.qlog` for forensic debugging of dropped connections — invaluable in Phase 5.

Total: ~11–12 engineer-days of networking work. Items 1, 4, 5, 6, 10 are on the critical path for Phase 2 sender/receiver code; the rest can land in parallel.

---

## Phase 5 risks worth budgeting now

**pion/ice ↔ quic-go integration is the riskiest plumbing in the project.** quic-go v0.59.1 takes a `net.PacketConn` via `quic.Transport.Conn`. pion/ice gives an `*ice.Conn` (a `net.Conn`). The adapter — one local-addr-faithful `net.PacketConn` per ICE-negotiated peer — is small but easy to get subtly wrong (deadline semantics, addr matching, EOF propagation). Budget a 3–5 day prototype spike at the start of Phase 5: two test processes on the same host, NAT simulated with `tc netem`, end-to-end QUIC over ICE, then send a 1 GB file. If this spike does not work in week 1 of Phase 5, the whole phase slips.

**The "75% ICE / 25% TURN" line is folklore.** Tailscale's published data ([blog post on NAT traversal](https://tailscale.com/blog/how-nat-traversal-works)) and large-scale WebRTC studies put symmetric-NAT + CGNAT failure rates at 20–40% depending on geography. Indian mobile carriers (Jio, Airtel) use CGNAT extensively; estimates for mobile traffic on Jio specifically run to 40–60% of subscribers behind CGNAT at any given time. The Hyderabad↔Amsterdam example in the plan will, in the worst case, send 100% of its traffic through TURN. **Budget the VPS bandwidth on a worst-case TURN fallback rate, not on a folklore 25%.** A single hour of an idle TURN-relayed connection costs only KB; a single hour of a 4 GB file transfer costs 4 GB at the relay. With 100 concurrent users transferring an average of 1 GB/day each, expect 30–50 GB/day egress at the relay. Document this in the deploy guide.

**Connection migration interacts badly with ICE.** RFC 9000 §9 lets a QUIC client move its 4-tuple and continue an existing connection. But when QUIC sits on top of an ICE-managed `PacketConn`, the local address quic-go sees does not change even when the physical network does — and the *remote* address inside ICE changes invisibly, defeating QUIC's path validation. Phase 5 should document that **network handover drops the connection** and bolt reconnects with a fresh ICE session. Doing in-place migration over ICE is out of scope for v1; revisit when there is a real user complaining.

**TURN bandwidth and allocation lifecycle is the operator's nightmare, not the protocol's.** RFC 8656 mandates a 10-minute allocation default with periodic refresh. coturn handles refresh; the bolt daemon's responsibility is to keep at least one packet flowing through the allocation every 5 minutes, otherwise the underlying NAT mapping for the *server-side* coturn-to-NAT-traversal-target can expire. quic-go's `KeepAlivePeriod` solves this end-to-end. Mention in the deploy guide that coturn's `--no-loopback-peers` and `--no-multicast-peers` should be set; that bandwidth quotas should be configured per relay_secret epoch; and that monthly relay_secret rotation forces a soft re-pairing of all peers.

**Symmetric-NAT detection happens before TURN fallback.** pion/ice will time out gathering server-reflexive candidates and fall back. Budget 10–20 s of connection-establishment latency on first contact for the worst-case CGNAT-to-CGNAT pairing. The user-visible "connecting…" spinner should accommodate this; today there is no such progress signal in the IPC events. Add a `connection_progress` IPC event in Phase 5 so the TUI can show "trying direct → trying TURN".

---

## References (RFCs and quic-go APIs)

**RFCs.**
- RFC 9000 — *QUIC: A UDP-Based Multiplexed and Secure Transport*. §2 streams, §8 address validation, §9 connection migration, §10.1 idle timeout, §14.1 initial packet size.
- RFC 9001 — *Using TLS to Secure QUIC*. §5.6 0-RTT replay protection.
- RFC 9221 — *Unreliable Datagram Extension to QUIC*.
- RFC 8445 — *Interactive Connectivity Establishment (ICE)*. ICE-lite vs full ICE, controlling/controlled role, candidate gathering.
- RFC 8489 — *Session Traversal Utilities for NAT (STUN)*.
- RFC 8656 — *Traversal Using Relays around NAT (TURN)*. Allocation lifetime, refresh.
- RFC 4787 — *NAT Behavioral Requirements for UDP*. Mapping refresh timer ≥ 2 min recommended; real-world routers often ignore this.
- RFC 5869 — *HKDF*.
- RFC 5705 — *Keying Material Exporters for TLS*.
- RFC 8470 — *Using Early Data in HTTP*. 0-RTT replay risks; same principles apply at QUIC layer.
- RFC 2104 — *HMAC*.

**quic-go v0.59.1 API references.**
- `quic.Config{KeepAlivePeriod, MaxIdleTimeout, HandshakeIdleTimeout, MaxIncomingStreams, MaxIncomingUniStreams, EnableDatagrams, Allow0RTT, DisablePathMTUDiscovery, InitialPacketSize, InitialStreamReceiveWindow, MaxStreamReceiveWindow, InitialConnectionReceiveWindow, MaxConnectionReceiveWindow, Tracer}`.
- `quic.Transport{Conn net.PacketConn, MaxUnvalidatedHandshakes int, VerifySourceAddress func(net.Addr) bool}`.
- `quic.Transport.Listen(*tls.Config, *quic.Config) (*quic.Listener, error)`.
- `quic.Transport.Dial(ctx, *net.UDPAddr, *tls.Config, *quic.Config) (*quic.Conn, error)`.
- `quic.Transport.DialEarly(...)` — *not used in v1; 0-RTT disabled.*
- `quic.ListenAddr(addr, *tls.Config, *quic.Config) (*quic.Listener, error)` — current code path.
- `quic.DialAddr(ctx, addr, *tls.Config, *quic.Config) (*quic.Conn, error)` — current code path.
- `quic.Conn.{OpenStreamSync, AcceptStream, SendDatagram, ReceiveDatagram, CloseWithError, ConnectionState}`.
- `quic.Stream.{CancelRead, CancelWrite, SetReadDeadline, SetWriteDeadline}`.
- `tls.Config.{VerifyPeerCertificate, ClientAuth, SessionTicketsDisabled, NextProtos, MinVersion}`. `crypto/tls.ConnectionState.ExportKeyingMaterial(label, context, length)` for RFC 5705 binding.

---

## Final user-visible summary

**Top 3 plan critiques (protocol level):**
1. `ChunkMsg.Data []byte` will base64-encode through `encoding/json`, adding 33% wire overhead and forcing a multi-MB unmarshal per chunk — gigabit LAN goal is unreachable. Switch to a header-then-raw-bytes frame on dedicated chunk streams.
2. `quic.Config` is under-specified: no explicit `MaxIdleTimeout`, no receive-window tuning, no `EnableDatagrams`, no `Allow0RTT` decision. Default windows leave a single QUIC stream below 250 Mbps on a 200 ms RTT path. Tune now; lock `Allow0RTT=false`.
3. TOFU has two real holes: server `ServerTLSConfig` does not run `VerifyPeerCertificate`; client `peerVerifier()` accepts every unknown peer instead of pinning on first use. Combined with the cert/key binding being construction-time (no Ed25519 proof-of-possession), the model is weaker than SSH's. Add a signature over RFC 5705 keying material in `HandshakeMsg`, and wire the prompt-on-first-use TOFU flow before Phase 2.

**Top 5 decisions the PO must lock before Phase 2:**
1. ChunkMsg encoding — header + raw bytes (recommended) vs JSON+base64.
2. Stream model — one-chunk-per-stream (recommended) vs N-streams-round-robin.
3. Enable QUIC datagrams in `quic.Config` now for forward compatibility with Phase 4 chat presence/typing/heartbeat.
4. `Allow0RTT=false` and `SessionTicketsDisabled=true` for v1.
5. Lock ALPN at `bolt/1`; lock idle timeouts (`MaxIdleTimeout=60s`, `KeepAlivePeriod=20s`); add Ed25519 proof-of-possession to `HandshakeMsg`.

**Top 5 Phase 2 networking prerequisites (≤ 12 engineer-days total):**
1. Tighten `quic.Config` defaults (timeouts, windows, datagrams, 0-RTT off). — 1 day.
2. Wire real TOFU verifier on both client and server `tls.Config` with first-use prompt and `peers.toml` pinning. — 2 days.
3. Add Ed25519 proof-of-possession to `HandshakeMsg`. — 1 day.
4. Decide and implement chunk framing; define `ChunkHeader`/`FileHeader`/`TransferAck`/`TransferDone` v1 schemas; freeze. — 2 days.
5. Stream router + `OpenChunkStream`/`AcceptChunkStream` helpers + `PeerConn.PathType` field + read/write deadlines + bounded receiver buffer pool. — 3 days.

**Full report:** `.agent/reports/networking-review.md`.
