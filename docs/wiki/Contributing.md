# Contributing

## Prerequisites

- Go 1.26+
- Node 20+

## Running locally

```bash
# backend
go run ./cmd/shortr

# frontend, in a second terminal (proxies /api and /auth to :8080)
cd web && npm install && npm run dev
```

## Tests

```bash
go vet ./...
go test ./... -race
cd web && npm run build   # type-checks, then builds
cd web && npm run lint
```

CI runs the same checks, plus a Docker build, on every push and pull request.

## Submitting changes

1. Fork the repo and create a branch.
2. Make your change and add tests where it makes sense.
3. Make sure the checks above pass.
4. Open a pull request that explains why the change is needed.

Please don't open public issues for security problems; see
[Security](Security.md) instead.

## Releasing (maintainers)

Pushing a version tag starts the release workflow. It builds the frontend,
cross-compiles the binaries for Linux, macOS and Windows, attaches them and
a `SHA256SUMS` file to a GitHub release, and publishes a multi-arch Docker
image to GHCR.

```bash
git tag v1.2.3
git push origin v1.2.3
```
