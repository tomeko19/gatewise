# 🔐 Gatewise

**Unified Access Layer for Modern Infrastructure**

Gatewise is an Open Source (Open Core) platform for centralized access and security management across Kubernetes, Kafka, and Kong. Define policies once, enforce everywhere.

![Version](https://img.shields.io/badge/version-0.3.0--alpha-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![React](https://img.shields.io/badge/React-18+-61DAFB?logo=react)

---

## ✨ Features

### 🎯 Core Features (Community Edition)
- **Unified Policy Management**: Define access policies in YAML
- **Kubernetes Orchestration**: Namespaces, RBAC, Resource Quotas
- **Multi-Connector Architecture**: Kubernetes, Kafka, Kong
- **REST API & CLI**: Full-featured API and command-line interface
- **Audit Logs**: Complete audit trail of all operations
- **Dashboard**: Beautiful React-based web interface

### 🚀 Advanced Features (Enterprise Edition)
- **Self-Healing (Anti-Drift)**: Automatic detection and correction of manual changes
- **Just-In-Time (JIT) Access**: Temporary, self-expiring access grants
- **Real-time Updates**: WebSocket-based live dashboard updates
- **Kafka & Kong Connectors**: Advanced integration with Kafka topics/ACLs and Kong Gateway
- **BYOI Support**: Bring Your Own Infrastructure model

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────┐
│           Gatewise Control Plane                │
│  ┌──────────────┐         ┌─────────────────┐  │
│  │  Dashboard   │◄────────┤   REST API      │  │
│  │   (React)    │         │  (Go + Python)  │  │
│  └──────────────┘         └─────────────────┘  │
│         ▲                         │             │
│         │ WebSocket               │             │
│         └─────────────────────────┘             │
└─────────────────────────────────────────────────┘
                    │
        ┌───────────┼───────────┐
        ▼           ▼           ▼
   Kubernetes     Kafka       Kong
   (Namespaces)  (Topics)   (Routes)
```

---

## 🚀 Quick Start

### Prerequisites
- **Go** 1.21+
- **Node.js** 18+ & Yarn
- **Python** 3.9+
- **PostgreSQL** (for production) or **MongoDB** (for dev)

### Installation

#### 1. Clone the repository
```bash
git clone https://github.com/yourusername/gatewise.git
cd gatewise
```

#### 2. Build the Go backend
```bash
cd gatewise
go mod tidy
go build -o bin/gatewise-server ./cmd/gatewise-server
go build -o bin/gatewise-cli ./cmd/gatewise-cli
```

#### 3. Install Python dependencies
```bash
cd ../backend
pip install -r requirements.txt
```

#### 4. Install frontend dependencies
```bash
cd ../frontend
yarn install
```

#### 5. Configure environment variables
```bash
# backend/.env
MONGO_URL="mongodb://localhost:27017"
DB_NAME="gatewise"
CORS_ORIGINS="*"

# frontend/.env
REACT_APP_BACKEND_URL=http://localhost:8001
```

#### 6. Start services
```bash
# Terminal 1: Start backend
cd backend
python server.py

# Terminal 2: Start frontend
cd frontend
yarn start
```

#### 7. Access the dashboard
Open http://localhost:3000 in your browser

---

## 📖 Usage

### Define a Policy (YAML)

Create a policy file `my-policy.yaml`:

```yaml
apiVersion: gatewise.io/v1
kind: Policy
metadata:
  name: my-team-access
spec:
  team: marketing
  kubernetes:
    namespaces:
      - name: marketing-prod
        quotas:
          cpu: "4"
          memory: "8Gi"
    rbac:
      - role: admin
        subjects:
          - kind: User
            name: alice@example.com
  kafka:
    topics:
      - name: marketing-events
        partitions: 3
        replication: 2
    acls:
      - principal: User:alice
        operations: [Read, Write]
  kong:
    routes:
      - name: marketing-api
        service: marketing-service
        paths: [/api/v1/marketing]
```

### Apply a Policy

#### Via CLI
```bash
./gatewise/bin/gatewise-cli policy apply -f my-policy.yaml
```

#### Via API
```bash
curl -X POST http://localhost:8001/api/policies \
  -H "Content-Type: application/json" \
  -d '{"yaml": "..."}'
```

#### Via Dashboard
1. Navigate to **Policies** page
2. Click **"+ New Policy"**
3. Paste your YAML
4. Click **"Validate"** then **"Create"**

---

## 🎯 JIT Access (Enterprise)

Request temporary access:

```bash
curl -X POST http://localhost:8001/api/jit/request \
  -H "Content-Type: application/json" \
  -d '{
    "policyName": "my-team-access",
    "grantee": "bob@example.com",
    "duration": "2h",
    "reason": "Incident response"
  }'
```

---

## 🛡️ Self-Healing (Enterprise)

Enable automatic drift detection:

```bash
# Simulate drift
curl -X POST http://localhost:8001/api/drift/simulate

# View detected drifts
curl http://localhost:8001/api/drift

# Repair specific drift
curl -X POST http://localhost:8001/api/drift/{drift-id}/repair

# Repair all drifts
curl -X POST http://localhost:8001/api/drift/repair-all
```

---

## 📦 Deployment

### Using Helm (Recommended)

```bash
cd gatewise/helm/gatewise

# Community edition
helm install gatewise . \
  --set license.tier=community

# Enterprise edition
helm install gatewise . \
  --set license.tier=enterprise \
  --set license.key=ENT-YOUR-LICENSE-KEY
```

### Using Docker Compose

```bash
docker-compose up -d
```

---

## 🔑 License Tiers

| Feature | Community | Enterprise |
|---------|-----------|------------|
| Kubernetes Basic RBAC | ✅ | ✅ |
| Kubernetes Namespaces | ✅ | ✅ |
| Kubernetes Quotas | ✅ | ✅ |
| Policy Parsing | ✅ | ✅ |
| Audit Logs | ✅ | ✅ |
| Kafka Topics & ACLs | ❌ | ✅ |
| Kong Routes & Services | ❌ | ✅ |
| Self-Healing (Anti-Drift) | ❌ | ✅ |
| JIT Access | ❌ | ✅ |
| WebSocket Real-time | ✅ | ✅ |

To upgrade to Enterprise, add your license key in **Settings** or via API:

```bash
curl -X POST http://localhost:8001/api/license \
  -H "Content-Type: application/json" \
  -d '{"key": "ENT-YOUR-LICENSE-KEY"}'
```

---

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🔗 Links

- **Documentation**: [docs.gatewise.io](https://docs.gatewise.io) *(coming soon)*
- **Issues**: [GitHub Issues](https://github.com/yourusername/gatewise/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourusername/gatewise/discussions)

---

## 🙏 Acknowledgments

Built with ❤️ using:
- [Go](https://golang.org/)
- [React](https://reactjs.org/)
- [FastAPI](https://fastapi.tiangolo.com/)
- [Kubernetes Client-Go](https://github.com/kubernetes/client-go)
- [Sarama (Kafka)](https://github.com/IBM/sarama)

---

**Made with [Emergent](https://emergent.sh)**
