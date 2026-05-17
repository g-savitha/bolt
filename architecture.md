# bolt — Architecture Diagram

## System Overview

```mermaid
graph TB
    subgraph "Your Machine (Hyderabad)"
        CLI["bolt CLI\ncmd/bolt/main.go"]
        SOCK["Unix Socket\ndaemon.sock"]
        DAEMON["Daemon\ninternal/daemon/daemon.go"]
        ID["Identity\nEd25519 Keypair"]
        CFG["Config\nconfig.toml + peers.toml"]
        REG["Peer Registry\nin-memory"]
    end

    subgraph "Transport Layer"
        QUIC["QUIC over UDP\nTLS 1.3 built-in\ninternal/transport/quic.go"]
        TLS["TLS + TOFU\ninternal/transport/tls.go"]
        PC["PeerConn\ninternal/transport/peer_conn.go"]
    end

    subgraph "Wire Protocol"
        PROTO["Stream Framing\ninternal/proto/wire.go"]
        S1["StreamHandshake 0x01"]
        S2["StreamChat 0x02"]
        S3["StreamFile 0x03"]
        S4["StreamControl 0x04"]
    end

    subgraph "Discovery"
        MDNS["mDNS\n_bolt._udp\nLAN only"]
        RELAY["Relay Server\nboltd\nInternet peers"]
        ICE["ICE Hole-punch\npion/ice"]
        TURN["TURN Fallback\ncoturn"]
    end

    subgraph "Friend's Machine (Amsterdam)"
        DAEMON2["bolt Daemon"]
        ID2["Ed25519 Identity"]
    end

    CLI -->|IPC commands| SOCK
    SOCK <-->|JSON frames| DAEMON
    DAEMON --> ID
    DAEMON --> CFG
    DAEMON --> REG
    DAEMON --> QUIC
    QUIC --> TLS
    TLS --> PC
    PC --> PROTO
    PROTO --> S1
    PROTO --> S2
    PROTO --> S3
    PROTO --> S4

    DAEMON -->|LAN| MDNS
    DAEMON -->|Internet| RELAY
    RELAY --> ICE
    ICE -->|fails 25 pct| TURN

    MDNS <-->|direct QUIC| DAEMON2
    ICE <-->|hole-punched QUIC| DAEMON2
    TURN <-->|relayed QUIC| DAEMON2
    DAEMON2 --> ID2
```

---

## Connection Paths

```mermaid
flowchart TD
    START([bolt send file friend]) --> CHECK{Same LAN?}

    CHECK -->|Yes| MDNS_PATH[mDNS Discovery\nauto-found in ~3 seconds]
    CHECK -->|No| RELAY_PATH[Register with\nRelay Server]

    MDNS_PATH --> DIRECT[Direct QUIC Connection\nwire speed, no relay involved]

    RELAY_PATH --> LOOKUP[Relay looks up\nfriend's IP and port]
    LOOKUP --> ICE_TRY[ICE Hole-punch attempt]

    ICE_TRY -->|Success 75 pct| DIRECT_NET[Direct QUIC\npeer-to-peer]
    ICE_TRY -->|Fail 25 pct| TURN_RELAY[TURN Relay\nyour VPS, encrypted]

    DIRECT --> AUTH
    DIRECT_NET --> AUTH
    TURN_RELAY --> AUTH

    AUTH[TLS 1.3 Handshake\nEd25519 certs] --> TOFU{Peer known?}

    TOFU -->|Yes, trusted| READY[Connection Ready]
    TOFU -->|Yes, blocked| REJECT[Connection Rejected]
    TOFU -->|New peer| PROMPT[Show fingerprint\nand randomart to user]
    PROMPT -->|User accepts| SAVE[Save to peers.toml\ntrust = allow-once]
    SAVE --> READY

    READY --> APPHS[Application Handshake\nverify Ed25519 matches TLS cert]
    APPHS --> TRANSFER[File Transfer\nor Chat]
```

---

## File Transfer Flow

```mermaid
sequenceDiagram
    participant S as Sender
    participant R as Receiver

    S->>R: StreamFile control - FileHeader with filename, size, hash, chunk_size

    alt Receiver has partial download
        R->>S: TransferAck accepted=true, resume_from=[3,7,12...]
    else New transfer
        R->>S: TransferAck accepted=true, resume_from=nil
    end

    note over S,R: 8 parallel QUIC streams, one per chunk goroutine

    par Chunk streams in parallel
        S->>R: StreamFile ChunkMsg index=0, hash, data
        S->>R: StreamFile ChunkMsg index=1, hash, data
        S->>R: StreamFile ChunkMsg index=2, hash, data
        S->>R: StreamFile ChunkMsg index=N, hash, data
    end

    S->>R: TransferDone with transfer_id
    note over R: Verify whole-file SHA-256\nRename temp file to destination
```

---

## TOFU Identity Verification

```mermaid
sequenceDiagram
    participant A as Alice (you)
    participant B as Bob (new peer)

    note over A,B: TLS 1.3 inside QUIC — channel encrypted

    A->>B: Self-signed cert with Ed25519 pubkey in SubjectKeyId
    B->>A: Self-signed cert with Ed25519 pubkey in SubjectKeyId

    note over A: Extract Bob's pubkey from cert\nCompute fingerprint

    alt Bob's fingerprint is in peers.toml
        note over A: Trust level check\nalways-allow: proceed\nallow-once: prompt\nblock: reject
    else Unknown fingerprint
        note over A: Show fingerprint and randomart\nInstruct user to verify out-of-band
        A->>A: User types y to accept
        note over A: Save Bob to peers.toml\ntrust = allow-once
    end

    note over A,B: Application handshake — both sides open StreamHandshake simultaneously

    A->>B: HandshakeMsg with public_key_hex, nickname, version
    B->>A: HandshakeMsg with public_key_hex, nickname, version

    note over A: Cross-check: pubkey in HandshakeMsg\nmust match SubjectKeyId in TLS cert\nMismatch means substitution attack — reject

    note over A,B: Connection fully authenticated\npeer_online event fired
```

---

## Data Storage

```mermaid
graph LR
    subgraph config ["~/.config/bolt/"]
        PK["identity/private.key\n0600 permissions\nEd25519 private key"]
        PUB["identity/public.key\n0644 permissions\nEd25519 public key"]
        CONF["config.toml\nversion=1\nnickname, port, receive_dir, relay"]
        PEERS["peers.toml\nversion=1\nfingerprint, trust level, nickname"]
        DSOCK["daemon.sock\nUnix socket at runtime"]
        DPID["daemon.pid\nflock-protected"]
    end

    subgraph share ["~/.local/share/bolt/"]
        CHAT["logs/peer-YYYY-MM-DD.log\nopt-in chat history"]
        XFER["logs/transfers-YYYY-MM-DD.log\nopt-in transfer history"]
        INC["incomplete/uuid.json\nresume state"]
        TMP["incomplete/uuid.tmp\npartial file data"]
    end
```

---

## Daemon Auto-Spawn (Race-Safe)

```mermaid
flowchart TD
    CMD([Any bolt command]) --> CHECK[Try connect to\ndaemon.sock]
    CHECK -->|Connected| READY2([Daemon already running\nSend IPC request])
    CHECK -->|Refused| LOCK[Acquire flock on\ndaemon.pid]
    LOCK --> RECHECK[Re-check socket\ninside lock]
    RECHECK -->|Now connected| UNLOCK[Release lock]
    UNLOCK --> READY2
    RECHECK -->|Still not running| SPAWN[Re-exec binary as\nbolt daemon\nSetsid=true detached]
    SPAWN --> WRITE[Write child PID\nto daemon.pid]
    WRITE --> POLL[Poll daemon.sock\nevery 50ms]
    POLL -->|Connected within 3s| UNLOCK
    POLL -->|Timeout| ERROR([Error: daemon did not start\nRun bolt daemon manually])
```
