# networking Status

**Last Updated**: 2026-05-19
**Status**: Reviewed
**Current Task**: None — review complete, awaiting PO decisions

---

## Top 5 Phase 2 networking prerequisites

1. **Tighten `quic.Config` defaults** — set `MaxIdleTimeout=60s`, `KeepAlivePeriod=20s`, `EnableDatagrams=true`, `Allow0RTT=false`, receive windows (4 MiB initial / 64 MiB max per stream; 8 MiB / 256 MiB per connection). ~1 day.
2. **Wire real TOFU verifier on both `ClientTLSConfig` and `ServerTLSConfig`** — first-use prompt via IPC, `peers.toml` pinning, server-side `VerifyPeerCertificate` (today it is missing — server accepts any client cert). ~2 days.
3. **Ed25519 proof-of-possession in `HandshakeMsg`** — sign `tls.ConnectionState.ExportKeyingMaterial("bolt-handshake", nil, 32)` (RFC 5705) and verify against `SubjectKeyId`. Closes the cert↔key binding gap. ~1 day.
4. **Decide and freeze chunk framing + v1 schemas** — `ChunkHeader` (JSON) + raw bytes on dedicated chunk streams (drop `ChunkMsg.Data []byte`). Freeze `FileHeader`/`TransferAck`/`ChunkHeader`/`TransferDone`. ~2 days.
5. **Stream router + helpers** — `daemon.routeStream` dispatch for `StreamFile`, `PeerConn.OpenChunkStream`/`AcceptChunkStream`, `PeerConn.PathType` field defaulting to `PathLAN`, stream read/write deadlines, bounded receiver buffer pool sized to `2 × N_parallel_streams`. ~3 days.

Total critical-path: ~9 days. Items 1, 4, 5 block sender/receiver coding.

---

## Top 3 plan critiques

1. **`ChunkMsg.Data []byte` ⇒ JSON base64** — ~33% wire overhead and a full unmarshal per chunk. Gigabit-LAN throughput goal is unreachable with the current shape. Switch to header-then-raw-bytes on dedicated chunk streams.
2. **`quic.Config` is under-specified** — no `MaxIdleTimeout`, no receive-window tuning, no `EnableDatagrams`, no explicit `Allow0RTT` decision. Default 6 MiB max-stream-window caps a single stream at ~30 MB/s on a 200 ms RTT path. Tune before Phase 2 freeze.
3. **TOFU plumbing is incomplete and partially broken** — `ServerTLSConfig` does not install `VerifyPeerCertificate` (server accepts any client cert until application-layer check); `peerVerifier()` accepts all unknown peers (no first-use prompt, no pinning). Cert↔Ed25519 binding is construction-time only with no proof-of-possession. Three fixes required before Phase 2 sender/receiver code is trusted.

---

## Blockers

None on the networking side. Awaiting PO sign-off on the 10 decisions in the full report.

## Recent Output

Full review: [.agent/reports/networking-review.md](../reports/networking-review.md) (2026-05-19).

## Next Steps

1. PO locks the 10 binary/multi-choice decisions in the "Decisions" section of the full report.
2. Networking opens 11 backlog stories from the "Phase 2 prerequisites" section.
3. Phase 5 prep: schedule a 3–5 day pion/ice ↔ `quic.Transport` plumbing spike at Phase 5 kickoff.
