# Gatewise - Enterprise Code Separation Guide

## Files Moved to Enterprise Repo

### Connectors (Private)
- `internal/connector/kafka.go` → `enterprise/connectors/kafka.go`
- `internal/connector/kong.go` → `enterprise/connectors/kong.go`

### Features (Private)
- `internal/jit/` → `enterprise/jit/`
- `internal/reconciler/drift.go` → `enterprise/drift/detector.go`

### License Validation (Private)
- Advanced license validation logic (server-side)

## Community Edition Stubs

Files created for Community Edition:
- `internal/connector/enterprise_stubs.go` - Returns error messages for Enterprise features

## Build Tags

### Community Build (default)
```bash
go build ./cmd/gatewise-server
```

### Enterprise Build (requires enterprise repo)
```bash
# Link enterprise code
ln -s /path/to/gatewise-enterprise enterprise

# Build with tags
go build -tags enterprise ./cmd/gatewise-server
```

## Repository Structure

### gatewise-io/gatewise (Public - MIT)
```
gatewise/
├── cmd/
├── internal/
│   ├── api/
│   ├── connector/
│   │   ├── factory.go
│   │   ├── kubernetes.go
│   │   └── enterprise_stubs.go    # ← Stubs
│   ├── operator/
│   └── reconciler/
├── pkg/
└── LICENSE (MIT)
```

### gatewise-io/gatewise-enterprise (Private)
```
gatewise-enterprise/
├── connectors/
│   ├── kafka.go
│   └── kong.go
├── jit/
│   └── manager.go
├── drift/
│   └── detector.go
└── LICENSE (Proprietary)
```

## Testing

### Community Edition
```bash
# All tests should pass without Enterprise code
go test ./...
```

### Enterprise Edition
```bash
# Requires enterprise directory
go test -tags enterprise ./...
```

## GitHub Actions

Separate workflows for Community and Enterprise:
- `.github/workflows/community-release.yml` - Public releases
- `.github/workflows/enterprise-build.yml` - Private builds (in enterprise repo)

## Distribution

### Community
- GitHub Releases: Public binaries
- Docker Hub: `gatewise/agent:latest`
- Helm: `gatewise/gatewise`

### Enterprise
- Private registry: `registry.gatewise.io/gatewise:enterprise`
- Customer portal downloads
- Helm: `gatewise/gatewise-enterprise`
