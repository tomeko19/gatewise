# Gatewise Community Edition

**Unified Access Layer for Kubernetes**

Gatewise is an Open Source platform for centralized access and security management for Kubernetes clusters.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://golang.org/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

---

## ✨ Community Features

- **Kubernetes Orchestration**: Namespaces, RBAC, Resource Quotas
- **Policy as Code**: Define access policies in YAML
- **GitOps Ready**: Kubernetes Operator with CRD support
- **REST API & CLI**: Full-featured API and command-line interface
- **Prometheus Metrics**: Built-in observability
- **Audit Logs**: Complete audit trail of all operations
- **Dashboard**: React-based web interface

## 🚀 Enterprise Features

Upgrade to [Gatewise Enterprise](https://gatewise.io/enterprise) for:

- 🎯 **Multi-Cluster Management**: Manage multiple Kubernetes clusters
- 📊 **Advanced Connectors**: Kafka Topics/ACLs, Kong Gateway Routes
- 🔐 **Just-In-Time Access**: Temporary, self-expiring access grants
- 🛡️ **Self-Healing**: Automatic drift detection and correction
- 🏢 **SSO/SAML**: Enterprise authentication
- 📞 **Priority Support**: 24/7 enterprise support

[Contact Sales](https://gatewise.io/contact)

---

## 📦 Quick Start

### Prerequisites

- Go 1.23+
- Kubernetes cluster
- PostgreSQL or MongoDB

### Installation

#### Via Helm

```bash
helm repo add gatewise https://gatewise-io.github.io/gatewise-helm
helm install gatewise gatewise/gatewise
```

#### Via Binary

```bash
# Download latest release
wget https://github.com/gatewise-io/gatewise/releases/latest/download/gatewise-linux-amd64

# Run the server
./gatewise-linux-amd64 server --db-url "postgresql://..."
```

---

## 📖 Documentation

- [Getting Started](https://docs.gatewise.io/getting-started)
- [API Reference](https://docs.gatewise.io/api)
- [GitOps Guide](GITOPS.md)
- [Monitoring](MONITORING.md)

---

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

**Enterprise features** are proprietary and require a commercial license.

---

## 🔗 Links

- **Website**: [gatewise.io](https://gatewise.io)
- **Documentation**: [docs.gatewise.io](https://docs.gatewise.io)
- **Helm Charts**: [gatewise-io/gatewise-helm](https://github.com/gatewise-io/gatewise-helm)
- **Enterprise**: [Contact Sales](https://gatewise.io/contact)

---

**Made with ❤️ by the Gatewise team**
