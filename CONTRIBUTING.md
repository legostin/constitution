# Contributing to Constitution

## Development

```bash
git clone https://github.com/legostin/constitution.git
cd constitution
make build
make test
```

## Running Tests

```bash
make test       # Unit tests with race detector
make e2e        # E2E tests (builds binary, tests against real config)
make lint       # go vet
```

## Making Changes

1. Create a branch from `main`
2. Make your changes
3. Run `make test && make e2e`
4. Commit with a clear message
5. Open a PR

## Code Structure

```
cmd/constitution/     CLI + hook handler
cmd/constitutiond/    Remote service
internal/check/       Check type implementations (10 types)
internal/config/      Config loading and merging
internal/engine/      Rule evaluation engine
internal/handler/     Hook event handlers
internal/hook/        JSON I/O + rule filtering
internal/celenv/      CEL expression environment
internal/plugin/      Plugin system (exec/http)
internal/remote/      Remote API client
internal/server/      HTTP server for constitutiond
pkg/types/            Shared types (HookInput, Rule, etc.)
e2e/                  End-to-end tests
configs/              Template configs + orchestration patterns
```

## Adding a Check Type

1. Create `internal/check/your_check.go` implementing the `Check` interface
2. Register in `internal/check/registry.go`
3. Add tests in `internal/check/your_check_test.go`
4. Document in README.md under "Check Types"

## Adding an Orchestration Pattern

1. Create `cmd/constitution/configs/your-pattern.yaml`
2. Add embed in `cmd/constitution/embed.go`
3. Add to `workflowTemplates` map in `embed.go`
4. Update `init_cmd.go` interactive menu
5. Document in README.md under "Orchestration Patterns"

## Release Process

Tags trigger releases via GitHub Actions + goreleaser:

```bash
git tag v1.0.0
git push origin v1.0.0
```
