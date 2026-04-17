# SCION Path-Aware Networking Client

## What is SCION?

**SCION** (Scalability, Control, and Isolation On Next-generation Networks) is a next-generation Internet architecture designed from the ground up to address the fundamental security, scalability, and availability limitations of today's Internet. It was conceived and developed by Prof. Adrian Perrig and his team at ETH Zürich, with the reference implementation maintained at [scionproto/scion](https://github.com/scionproto/scion).

### The Problem with Today's Internet

The current Internet relies on the **Border Gateway Protocol (BGP)** for inter-domain routing. BGP was designed for reachability — not security. It has no built-in mechanism to authenticate route announcements, which has led to decades of route hijackings, prefix leaks, and traffic interception attacks. Beyond security, BGP offers no path transparency: end hosts have no visibility into, or control over, the paths their packets take through the network.

### How SCION Works

SCION replaces BGP with a clean-slate routing architecture built around three core principles:

**1. Isolation Domains (ISDs)**

The Internet is partitioned into **Isolation Domains (ISDs)** — groups of Autonomous Systems (ASes) that share a common trust root. Each ISD is governed by a **Trust Root Configuration (TRC)**, a cryptographically signed policy document that names the set of trusted certificate authorities within that domain. This provides strong trust isolation: a compromise in one ISD cannot propagate to another.

**2. Path-Aware Networking**

Unlike BGP, SCION makes network paths explicit and visible to end hosts. Paths are discovered through a **beaconing** process: Core ASes (the trust anchors of each ISD) periodically originate **Path Construction Beacons (PCBs)** that flood through the network. Each AS on the path appends a cryptographically signed **hop field** before forwarding the beacon to its neighbors. The result is a set of authenticated path segments (up-segments, core-segments, down-segments) that can be assembled into end-to-end forwarding paths.

End hosts — not routers — select which path their traffic takes. This gives applications the ability to optimize for latency, bandwidth, geographic constraints, carbon footprint, or any other metric exposed in path metadata.

**3. Cryptographic Path Authentication**

Every hop field in a SCION path is protected by a **MAC** (using CMAC with AES) derived from the AS's secret key via the **DRKey** (Derivable Route-specific Key) infrastructure. DRKey is a hierarchical key derivation system that allows any AS to efficiently derive symmetric keys shared with any other AS or end host — without prior key exchange. This makes it computationally infeasible to forge or alter a path, eliminating entire classes of routing attacks.

### Additional Security Properties

- **EPIC (Every Packet Is Checked):** An extension that embeds per-packet source authentication MACs directly in the packet header, allowing on-path routers to verify packet origin without state.
- **Hidden Paths:** Restricts path segment distribution to authorized parties, enabling private network overlays.
- **FABRID (Flexible AS-level Border Router Interface for Traffic Engineering with Defined Policies):** Enables per-flow, interface-level traffic engineering by attaching policy indices to ingress/egress interface pairs. Network operators can define which physical links specific traffic classes may traverse, enforcing compliance, performance, or business constraints.

---

## Project Overview

This project implements a **path-aware SCION client** and extends the SCION control plane with support for advanced routing, traffic engineering, and cryptographic authentication features. It is built against a live multi-AS SCION topology spanning two ISDs and seven ASes.

The client communicates with a remote verifier over the SCION network, executing a suite of end-to-end tests that exercise path selection, policy enforcement, and EPIC/FABRID functionality.

---

## Architecture

### Topology

```
ISD 1                         ISD 2
┌──────────────────────┐     ┌──────────────────────────┐
│  AS 1-ff00:0:110     │     │  AS 2-ff00:0:210 (core)  │
│     (core)           │─────│                          │
│                      │     │  AS 2-ff00:0:211          │
│  AS 1-ff00:0:111     │     │  AS 2-ff00:0:212          │
│  AS 1-ff00:0:112     │     │  AS 2-ff00:0:213          │
│  AS 1-ff00:0:113     │     └──────────────────────────┘
└──────────────────────┘
```

Each AS runs a **control service** (beacon and path management), a **border router** (data-plane forwarding), and a **SCION daemon** (local path API accessible over gRPC).

---

## Features Implemented

### Path Discovery and Multi-Criteria Selection

The client connects to the local SCION daemon via gRPC and queries available paths to the remote verifier. Path metadata — latency per hop, bandwidth per link, and per-hop carbon intensity — is extracted from path segment extensions and used to drive selection algorithms:

| Test | Objective |
|------|-----------|
| `BasicConnectivityTest` | Establish a SCION path and exchange a message |
| `BasicMultipathTest` | Enumerate all available disjoint paths |
| `MinimizeCarbonIntensity` | Select the path minimizing aggregate carbon intensity |
| `MaximizeBandwidthWithBoundedLatency` | Maximize bandwidth subject to a hard latency SLA |

The bandwidth-latency optimizer performs a multi-pass filter: it first eliminates paths that exceed the latency bound, then selects those with the fewest missing bandwidth annotations, and finally picks the path with maximum bottleneck bandwidth.

### FABRID Traffic Engineering

[FABRID](https://netsec.ethz.ch/publications/papers/2023-fabrid.pdf) enables per-flow, interface-level policy enforcement. Each AS publishes a map from `(ingress, egress)` interface pairs to policy indices. The client parses FABRID query expressions and evaluates them against per-hop interface metadata to select compliant paths.

Supported expression syntax:
- **Conjunctive constraints** (`+`): all sub-expressions must be satisfied
- **Conditional alternatives** (`? : REJECT`): use a policy if available, reject otherwise
- **AS-scoped selectors** (`ISD-AS#ingress,egress@Policy`): target specific hops

The `FABRIDDataplanePath` is then constructed with the resolved policy IDs and DRKey-derived per-hop MACs, enforcing the selected policy in the data plane.

### EPIC Hidden Paths

The client requests EPIC-authenticated hidden paths via `PathReqFlags{Hidden: true}`. When available, it constructs an `EPICDataplanePath` that embeds per-packet source authentication MACs, enabling on-path routers to verify packet origin. If no hidden paths are available, it falls back to standard paths.

### Beacon Selection (Control Plane)

The control plane implements a diversity-aware beacon selection algorithm: it selects the `k−1` shortest beacons plus one maximally diverse beacon (maximizing unique link count across the selected set). This balances path efficiency with redundancy.

### DRKey Infrastructure

The control service implements **Level 1 DRKey** derivation — AS-to-AS symmetric keys derived from per-AS master secrets using a PRF. Keys are epoch-based with prefetching before expiry, and an LRU cache minimizes latency on the critical path. The `DRKeyInterService` gRPC endpoint serves keys to requesting control services.

### Trust and Certificate Infrastructure

SCION's PKI is anchored in TRCs. The implementation includes:
- TRC loading and validation against stored certificate chains
- ECDSA signing key management with a 5-second refresh cache (`CachingSignerGen`)
- Mutual TLS certificate loading for inter-AS gRPC (server and client authentication)
- Certificate renewal via local `ChainBuilder` or delegated CA with JWT authentication

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.21 |
| Networking | SCION (`scionproto/scion`), UDP over SCION dataplane |
| RPC | gRPC (`google.golang.org/grpc` v1.63) |
| Transport | QUIC (`quic-go` v0.43) |
| Inter-AS Auth | Mutual TLS (ECDSA certificates) |
| Path Signing | ECDSA with SHA-256 / SHA-512 |
| Hop Field Auth | CMAC-AES (`dchest/cmac`) |
| Key Derivation | DRKey (PRF-based hierarchical derivation) |
| Serialization | Protocol Buffers (`google.golang.org/protobuf`) |
| Storage | SQLite — BeaconDB, PathDB, TrustDB, DRKey stores (`modernc.org/sqlite`) |
| Metrics | Prometheus (`prometheus/client_golang`) |
| Tracing | Jaeger / OpenTracing |
| Logging | Zap (`go.uber.org/zap`) |
| Build | Bazel (reference implementation), shell scripts (client) |

---

## Repository Structure

```
.
├── project/main.go               # Client application — path discovery and test scenarios
├── lib/structures.go             # Shared types and test communication protocol
├── config/                       # Verifier configuration (TOML) and test specs (JSON)
├── scion/                        # Modified SCION reference implementation
│   └── control/
│       ├── cmd/control/main.go   # Control service entry point
│       ├── beaconing/            # Beacon origination, propagation, selection
│       ├── segreq/               # Path segment lookup and registration
│       ├── fabrid/               # FABRID policy engine and gRPC service
│       ├── drkey/                # DRKey derivation, caching, gRPC service
│       └── trust/                # TRC, certificate, and signer management
├── topology_storage/             # Generated AS topology files
├── build.sh                      # Build script (client-app + verifier-app)
├── start.sh / stop.sh            # Control service lifecycle management
└── startTopology.sh              # Multi-AS topology bootstrap
```

---

## Build & Run

### Prerequisites

- Go 1.21+
- Linux (amd64 or arm64), or a Lima/QEMU VM for the SCION topology
- A running SCION topology (provided via `startTopology.sh`)

### Build

```bash
./build.sh
```

Compiles `client-app` from `project/main.go` and downloads the pre-built `verifier-app` binary for the detected architecture.

### Start the Topology

```bash
./startTopology.sh
./start.sh
```

### Run the Client

```bash
./client-app -local <local-ip> -remote <verifier-scion-address>
```

Example:
```bash
./client-app -local 127.0.0.1 -remote 1-ff00:0:112,127.0.0.2:40000
```

### Stop

```bash
./stop.sh
./stopTopology.sh
```

---

## Test Scenarios

| ID | Test | Description |
|----|------|-------------|
| 01 | `BasicConnectivityTest` | Establish a SCION path and exchange a message |
| 02 | `BasicMultipathTest` | Enumerate all available disjoint paths |
| 10 | `MinimizeCarbonIntensity` | Select path minimizing aggregate carbon intensity |
| 11 | `MaximizeBandwidthWithBoundedLatency` | Maximize bandwidth under a latency SLA |
| 20 | `EpicHiddenPathTest` | Discover and use EPIC-authenticated hidden paths |
| 30 | `FabridConnectivityTest` | Verify basic FABRID policy-based path selection |
| 31–33 | `FabridPolicy1/2/3Test` | Evaluate conjunctive and conditional FABRID constraints |
| 40 | `ASFinderTest` | Discover reachable ASes via path enumeration |

---

## References

- [SCION Architecture](https://scion-architecture.net)
- [The SCION Internet Architecture (Perrig et al., ETH Zürich)](https://www.scion-architecture.net/pdf/SCION-book.pdf)
- [FABRID: Flexible AS-level Traffic Engineering (2023)](https://netsec.ethz.ch/publications/papers/2023-fabrid.pdf)
- [DRKey: Efficient Symmetric Key Distribution (2017)](https://scion-architecture.net/pdf/2017-drkey.pdf)
- [EPIC: Every Packet Is Checked (2021)](https://scion-architecture.net/pdf/2021-epic.pdf)
- [SCION Reference Implementation](https://github.com/scionproto/scion)
