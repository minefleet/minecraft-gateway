# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**minecraft-gateway** is a Kubernetes-native gateway controller for Minecraft servers. It routes Minecraft Java Edition connections based on hostname extracted from the handshake packet. The project has two main components:

1. **Kubernetes Controller** (Go) — reconciles CRDs and synchronizes routing configuration to dataplanes
2. **Edge Proxy Filter** (Rust) — an Envoy dynamic module that parses Minecraft handshakes and routes connections

## Common Commands

### Go Controller

```bash
make controller-build          # Compile Go binary
make manifests                 # Regenerate CRDs and RBAC from types (run after editing api/)
make generate                  # Regenerate DeepCopy methods (run after editing api/)
make fmt                       # go fmt
make vet                       # go vet
make lint                      # golangci-lint run
make lint-fix                  # golangci-lint run --fix
make test                      # Unit tests (uses envtest)
make test-e2e                  # E2E tests against Kind cluster
```

### Rust Edge Module

```bash
cargo build                                                          # Debug build
cargo build --release                                                # Optimized build
cargo zigbuild --target aarch64-unknown-linux-gnu --release          # Cross-compile for arm64
cargo zigbuild --target x86_64-unknown-linux-gnu --release           # Cross-compile for amd64
```

### Docker

```bash
make controller-docker-build   # Build controller image
make edge-docker-build         # Build edge (Envoy+filter) image
make controller-docker-buildx  # Multi-platform controller build
make edge-docker-buildx        # Multi-platform edge build
```

### Running a single Go test

```bash
go test ./internal/dataplane/edge/... -run TestFunctionName -v
# Or with ginkgo:
go test ./internal/controller/... -v --ginkgo.focus="describe block name"
```

### Local edge testing

```bash
cd test/edge && docker compose up  # Runs Envoy with the filter loaded
```

## Architecture

### Control Plane (Go)

- **`cmd/main.go`** — entry point; initializes controller-runtime manager, registers reconcilers
- **`api/v1/`** — CRD type definitions: `MinecraftJoinRoute`, `MinecraftFallbackRoute`, `MinecraftServerDiscovery`
- **`internal/controller/`** — five reconcilers (GatewayClass, Gateway, MinecraftJoinRoute, MinecraftFallbackRoute, MinecraftServerDiscovery)
- **`internal/dataplane/`** — `Dataplane` interface with `SyncGateway(name, routes, backends)`. Two implementations:
  - `EdgeDataplane` — builds domain snapshots and manages Envoy proxy lifecycle
  - `NetworkDataplane` — stub for a future implementation
- **`internal/dataplane/edge/`** — domain extraction/deduplication logic and proxy lifecycle (`proxy.go`), ADS stub (`ads.go`)

The GatewayReconciler drives dataplane sync: it collects all routes and backends for a Gateway, then calls `Dataplane.SyncGateway`.

### Data Plane (Rust — `crates/minefleet-edge/`)

An Envoy dynamic module (cdylib) that implements a listener filter:

- **`src/lib.rs`** — Minecraft handshake parser (packet 0x00, VarInt-length-prefixed) + domain-to-cluster mapper
- **`src/filter_state.rs`** — helpers for writing Envoy dynamic metadata

The filter reads bytes from the downstream connection, parses the Minecraft handshake `server_address` field, matches it against configured `domain_mappings` (with wildcard `*.suffix` support), then writes the target filter chain name into Envoy dynamic metadata so Envoy can select the correct filter chain.

**ABI note:** The Rust SDK is pinned to a specific git commit that matches Envoy **v1.37.1**. Do not update the SDK without updating the Envoy base image to the corresponding version.

### Key Flows

1. **Route sync:** CRD change → reconciler → `dataplane.SyncGateway` → domain snapshot pushed to Envoy via ADS (future) or local config
2. **Connection routing:** TCP connection → Envoy listener → minefleet-edge filter parses handshake → sets `dev.minefleet.edge/selected_filter_chain` metadata → Envoy picks filter chain → upstream cluster

## Code Generation

After editing types in `api/v1/`, always run:
```bash
make manifests generate
```

This regenerates CRDs (`config/crd/`), RBAC markers, and DeepCopy methods.

## Testing Notes

- Unit tests use `envtest` (real K8s API server binary downloaded by `make test`)
- Tests use Ginkgo v2; focus with `--ginkgo.focus`
- E2E tests require Docker (Kind cluster named `minecraft-gateway-test-e2e`)
- `CERT_MANAGER_INSTALL_SKIP=true` skips cert-manager installation in E2E

## Dependency Constraints

- Go version: 1.25+
- Envoy version for edge: **v1.37.1** (Rust SDK ABI-locked to this version)
- Rust edition: 2024
- Cross-compilation for edge uses `cargo-zigbuild` (available in the Docker builder image)
- 
# Mintlify documentation

## Working relationship
- You can push back on ideas-this can lead to better documentation. Cite sources and explain your reasoning when you do so
- ALWAYS ask for clarification rather than making assumptions
- NEVER lie, guess, or make up anything

## Project context
- Format: MDX files with YAML frontmatter
- Config: docs.json for navigation ONLY
- MDX files in docs/ folder
- Components: Mintlify components

## Content strategy
- Document just enough for user success - not too much, not too little
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
- Make assumptions - always ask for clarification
