# Contributing to minecraft-gateway

Thanks for your interest in contributing. This document covers how to get started, what to expect from the process, and how to propose larger changes.

## Before you start

This project assumes familiarity with Kubernetes. You should be comfortable with concepts like controllers, CRDs, reconcilers, and the Gateway API before contributing to the Go controller. For the Rust edge module, familiarity with Envoy and its filter ABI is helpful.

## Opening an issue

Before writing any code, open an issue on GitHub describing the bug or feature. This gives maintainers a chance to confirm the problem, share relevant context, and align on the right approach before you invest time in an implementation.

For bug reports, include:
- What you expected to happen and what actually happened
- Kubernetes version, cluster setup, and any relevant CRD configuration
- Logs from the controller or edge proxy if applicable

For feature requests, describe the problem you are trying to solve and how you imagine it working.

## Discussing bigger changes

If you want to add a significant feature, change the API, or rework a component, **please discuss it on [Discord](https://discord.minefleet.dev) in #gateway-dev before opening a PR.** This avoids wasted effort if the direction doesn't fit the project, and gives maintainers a chance to share context that might not be obvious from the code.

Good candidates for a Discord discussion first:
- New CRD fields or changes to existing API types
- New dataplane implementations or changes to the dataplane interface
- Changes to how routing or discovery works
- Anything that affects the Envoy ABI or Rust filter

Small bug fixes, documentation improvements, and clearly scoped changes can go straight to a PR.

## Running tests

```sh
make test        # Unit tests (uses envtest)
make test-e2e    # E2E tests against a Kind cluster
```

## Local edge testing

```sh
cd test/edge && docker compose up
```

This runs Envoy with the filter loaded locally.

## Making changes

### API types

After editing types in `api/`, always regenerate:

```sh
make manifests generate proto
```

This regenerates CRDs under `config/crd/`, RBAC markers, and DeepCopy methods. Do not manually edit the `zz_generated.deepcopy.go` file.

### Code style

Before submitting a PR, run:

```sh
make fmt      # go fmt
make vet      # go vet
make lint     # golangci-lint
```

Fix any lint issues before opening the PR. You can use `make lint-fix` for auto-fixable issues.

## Opening a pull request

- Keep PRs focused. One concern per PR makes review faster.
- Describe what changed and why, not just what the diff does.
- If your PR changes the API or behavior, update the relevant samples under `config/samples/`.
- If you discussed the change on Discord first, mention it in the PR description.

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
