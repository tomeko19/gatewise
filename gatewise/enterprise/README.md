# Enterprise Code - Gatewise

This directory contains proprietary Enterprise features.

## Structure

```
enterprise/
├── connectors/
│   ├── kafka.go          # Kafka Topics & ACLs management
│   └── kong.go           # Kong Gateway routes & services
├── jit/
│   └── manager.go        # Just-In-Time access manager
├── drift/
│   └── detector.go       # Self-healing drift detection
├── multicluster/
│   └── orchestrator.go   # Multi-cluster orchestration
└── license/
    └── validator.go      # License validation server
```

## Building

Enterprise features are compiled separately and linked at build time:

```bash
# Build with Enterprise features
go build -tags enterprise -o gatewise-enterprise ./cmd/gatewise-server
```

## License Validation

Enterprise features require a valid license key. The validation happens server-side:

```go
if !license.ValidateEnterprise(key) {
    return errors.New("invalid enterprise license")
}
```

## Distribution

Enterprise binaries are distributed through:
- Private Docker registry: `registry.gatewise.io/gatewise-enterprise:latest`
- Direct download for licensed customers
- Helm chart with Enterprise flag: `--set enterprise.enabled=true`

---

**For licensing information, contact: sales@gatewise.io**
