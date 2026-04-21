# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**minecraft-gateway** is a Kubernetes-native gateway controller for Minecraft servers. It routes Minecraft Java Edition connections based on hostname extracted from the handshake packet. The project has three main components:

1. **Kubernetes Controller** (Go) — reconciles CRDs and synchronizes routing configuration to dataplanes
2. **Edge Proxy Filter** (Rust) — an Envoy dynamic module that parses Minecraft handshakes and routes connections
3. **Network Integration** (Java/Gradle) — a Velocity plugin + API library for network-side proxy integration

## Common Commands

### Go Controller

```bash
make controller-build          # Compile Go binary to bin/manager
make manifests                 # Regenerate CRDs and RBAC from types (run after editing api/)
make generate                  # Regenerate DeepCopy methods + proto (run after editing api/ or proto files)
make proto                     # Generate Go and Java from proto files via buf
make fmt                       # go fmt ./...
make vet                       # go vet ./...
make lint                      # golangci-lint run
make lint-fix                  # golangci-lint run --fix
make test                      # Unit tests (uses envtest) + network Java tests
make test-e2e                  # E2E tests against Kind cluster
make setup-test-e2e            # Create Kind cluster for E2E tests
make cleanup-test-e2e          # Delete Kind cluster
```

### Rust Edge Module

```bash
cargo build                                                          # Debug build
cargo build --release                                                # Optimized build
cargo zigbuild --target aarch64-unknown-linux-gnu --release          # Cross-compile for arm64
cargo zigbuild --target x86_64-unknown-linux-gnu --release           # Cross-compile for amd64
```

### Java Network Integration

```bash
make network-build             # Build Java API library via Gradle
make network-test              # Run Java unit tests (Gradle :api:test)
make network-publish           # Publish Java API to Maven Central
```

### Docker

```bash
make controller-docker-build   # Build controller image
make edge-docker-build         # Build edge (Envoy+filter) image
make network-docker-build      # Build all network integration images
make docker-build              # Build all three images
make controller-docker-buildx  # Multi-platform controller build+push
make edge-docker-buildx        # Multi-platform edge build+push
make docker-buildx             # Multi-platform build+push of all images
```

### Running a single Go test

```bash
go test ./internal/dataplane/edge/... -run TestFunctionName -v
# Or with ginkgo:
go test ./internal/controller/... -v --ginkgo.focus="describe block name"
```

### Local edge testing

```bash
cd test/edge && docker compose up  # Runs Envoy with the filter against two Minecraft servers
```

### Cluster deployment

```bash
make install                   # Install CRDs into current cluster
make deploy                    # Deploy controller to current cluster
make build-installer           # Generate dist/install.yaml
```

## Architecture

### Control Plane (Go)

- **`cmd/main.go`** — entry point; initializes controller-runtime manager, registers reconcilers, logs build version info
- **`api/controller/v1alpha1/`** — CRD type definitions: `MinecraftJoinRoute`, `MinecraftFallbackRoute`, `NetworkInfrastructure`. Group: `gateway.networking.minefleet.dev`
- **`api/network/v1alpha1/`** — protobuf definitions for the network xDS gRPC API (`NetworkXDS` service); source of truth for `buf generate`
- **`internal/controller/`** — five reconcilers:
  - `GatewayClassReconciler` — sets Accepted condition on GatewayClass
  - `GatewayReconciler` — **main orchestrator**; collects routes + backends for a Gateway, calls `Dataplane.SyncGateway`; owns the `Dataplane` and wires in `dataplane.Executor` as a Manager runnable
  - `MinecraftJoinRouteReconciler` — reconciles join routes
  - `MinecraftFallbackRouteReconciler` — reconciles fallback routes
  - `NetworkInfrastructureReconciler` — reconciles `NetworkInfrastructure`; resolves discovered Services into `status.backendRefs`
- **`internal/dataplane/`** — `Dataplane` interface (`SyncGateway`, `DeleteGateway`), composite fan-out implementation, `Executor` runnable (reads `POD_IP` env var at startup). Two implementations:
  - `EdgeDataplane` (`edge.go`) — manages per-gateway snapshot cache, drives edge xDS server
  - `NetworkDataplane` (`network.go`) — manages per-listener Velocity proxy Deployments+Services, drives network xDS server
- **`internal/dataplane/edge/`** — snapshot building (`snapshot.go`), domain deduplication (`domain.go`), DaemonSet + ConfigMap management (`proxy.go`), xDS server using go-control-plane (`xds.go`), ADS lifecycle (`ads.go`)
- **`internal/dataplane/network/`** — snapshot building (`snapshot.go`), per-listener Deployment+Service management (`proxy.go`), `NetworkXDS` gRPC server implementation (`ads.go`)
- **`internal/gateway/`** — infrastructure merging, GatewayClass index, NetworkInfrastructure index
- **`internal/route/`** — route filtering by listener (`AllowedRoutes`), route indexing, reference verification; `Bag` holds `[]MinecraftJoinRoute` + `[]MinecraftFallbackRoute`
- **`internal/discovery/`** — helpers for resolving `NetworkInfrastructure` by gateway or service
- **`internal/endpoint/`** — helpers for resolving `EndpointSlice` by service
- **`internal/version/`** — build-time `Version`, `CommitSHA`, `BuildDate` vars (injected via `-ldflags`)

The `GatewayReconciler` drives dataplane sync: it collects all routes and backends for a Gateway, then calls `Dataplane.SyncGateway`.

### Data Plane (Rust — `crates/minefleet-edge/`)

An Envoy dynamic module (`cdylib`) implementing two filters:

- **`src/lib.rs`** — `envoy_dynamic_module_on_program_init` entry point; registers the validator (listener filter) and router (network filter)
- **`src/validator.rs`** — `McRouterFilter` listener filter: parses Minecraft Java Edition handshake (packet 0x00, VarInt-length-prefixed), extracts `server_address`, performs exact + `*.suffix` wildcard lookup in `domain_mappings`, writes matched cluster name into Envoy dynamic metadata namespace `dev.minefleet.edge` key `cluster`. Config JSON: `default_server_name`, `domain_mappings`, `reject_unknown`, `max_scanned_bytes`, `metadata_namespace`, `metadata_key`. Exposes Prometheus counters `minecraft_router_matches_total` and `minecraft_router_misses_total`.
- **`src/router.rs`** — `McNetworkFilter` network filter: reads `dev.minefleet.edge/cluster` from dynamic metadata and writes it into `envoy.tcp_proxy.cluster` filter state so the tcp_proxy filter selects the correct upstream cluster.

**ABI note:** The Rust SDK is pinned to git commit `ce64b2ab5841d967886a7cbb3e99a37f9cd2c3a1`, which matches the Envoy base image in `Dockerfile.edge` (`envoyproxy/envoy:dev-ce64b2ab...`). These two pins must always match — do not update one without updating the other.

### Network Integration (Java — `integrations/`)

A Velocity plugin (`integrations/velocity/`) and a shared API library. The API library is published to Maven Central and consumed by server-side plugins. The Velocity proxy connects to the controller's network xDS server on port 19000.

### Key Flows

1. **Route sync:** CRD change → reconciler → `dataplane.SyncGateway` → edge snapshot pushed via xDS (port 18000) / network snapshot served via gRPC (port 19000)
2. **Connection routing (edge):** TCP connection → Envoy listener → `validator.rs` parses Minecraft handshake → writes cluster name to `dev.minefleet.edge/cluster` metadata → `router.rs` writes to `envoy.tcp_proxy.cluster` filter state → Envoy tcp_proxy routes to upstream cluster
3. **Connection routing (network):** Velocity proxy polls `NetworkXDS.GetSnapshot` → gets per-listener `ManagedService`/`ManagedServer` list → routes players accordingly

## Code Generation

After editing types in `api/controller/v1alpha1/`, always run:
```bash
make manifests generate
```

After editing proto files in `api/network/v1alpha1/`, run:
```bash
make proto
```

`make generate` calls both DeepCopy generation and proto generation. `make manifests` regenerates CRDs (`config/crd/`) and RBAC markers.

## Testing Notes

- Unit tests use `envtest` (real K8s API server binary downloaded by `make test`)
- Tests use Ginkgo v2; focus with `--ginkgo.focus`
- `make test` also runs Java network integration tests (`make network-test`)
- E2E tests require Docker (Kind cluster named `minecraft-gateway-test-e2e`)
- `CERT_MANAGER_INSTALL_SKIP=true` skips cert-manager installation in E2E
- Local edge testing: `cd test/edge && docker compose up` (two Paper Minecraft servers + Envoy with the filter)

## Documentation

Documentation lives in the `docs/` folder as Mintlify MDX files. **Documentation must be kept up to date** — any change that affects user-facing behavior, CRD fields, APIs, configuration options, or architectural flows must include a corresponding documentation update in the same commit or PR.

- **`docs/`** — all documentation pages (MDX format with YAML frontmatter required on every page)
- **`docs.json`** — navigation structure for the docs site (Mintlify config); edit this when adding or reorganizing pages

Current doc sections: Getting Started (`quickstart`), Concepts, Guides, Reference (one page per CRD).

## Dependency Constraints

- Go version: 1.25+
- Envoy version for edge: pinned to commit `ce64b2ab5841d967886a7cbb3e99a37f9cd2c3a1` (≈ v1.37.x); Rust SDK ABI-locked — must match Dockerfile.edge base image
- Rust edition: 2024
- Cross-compilation for edge uses `cargo-zigbuild` (available in the Docker builder image)
- Java: 25 (for network integration)
- Gateway API CRD version: v1.5.1

# Mintlify documentation

## Working relationship
- You can push back on ideas — this can lead to better documentation. Cite sources and explain your reasoning when you do so
- ALWAYS ask for clarification rather than making assumptions
- NEVER lie, guess, or make up anything

## Project context
- Format: MDX files with YAML frontmatter
- Config: docs.json for navigation ONLY
- MDX files in docs/ folder
- Components: Mintlify components

## Content strategy
- Document just enough for user success — not too much, not too little
- Prioritize accuracy and usability
- Make content evergreen when possible
- Search for existing content before adding anything new. Avoid duplication unless it is done for a strategic reason
- Check existing patterns for consistency
- Start by making the smallest reasonable changes

## docs.json

- Refer to the [docs.json schema](https://mintlify.com/docs.json) when building the docs.json file and site navigation

## Frontmatter requirements for pages
- title: Clear, descriptive page title
- description: Concise summary for SEO/navigation

## Writing standards
- Second-person voice ("you")
- Prerequisites at start of procedural content
- Test all code examples before publishing
- Match style and formatting of existing pages
- Include both basic and advanced use cases
- Language tags on all code blocks
- Alt text on all images
- Relative paths for internal links

## Git workflow
- NEVER use --no-verify when committing
- Ask how to handle uncommitted changes before starting
- Create a new branch when no clear branch exists for changes
- Commit frequently throughout development
- NEVER skip or disable pre-commit hooks

## Do not
- Skip frontmatter on any MDX file
- Use absolute URLs for internal links
- Include untested code examples
- Make assumptions — always ask for clarification