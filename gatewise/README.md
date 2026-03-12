# Gatewise

**The Unified Access Layer for Modern Infrastructure**

Gatewise est une plateforme Open Source (Open Core) qui permet de gérer les accès et la sécurité des infrastructures modernes (Kubernetes, Kafka, Kong) de manière centralisée.

## 🎯 Vision Enterprise

### BYOI - Bring Your Own Infrastructure
Gatewise est conçu comme un orchestrateur "agnostique" :
- **Mode Embedded** : Gatewise déploie et gère l'infrastructure (via Helm)
- **Mode External** : Gatewise se connecte à l'infrastructure existante du client

### Tiering (Licences)
| Feature | Community | Enterprise |
|---------|-----------|------------|
| K8s Namespace | ✓ | ✓ |
| K8s RBAC | ✓ | ✓ |
| K8s Quotas | ✓ | ✓ |
| Audit Log | ✓ | ✓ |
| Kafka Connector | ✗ | ✓ |
| Kong Connector | ✗ | ✓ |
| Keycloak SSO | ✗ | ✓ |
| Cerbos Authz | ✗ | ✓ |
| Self-Healing | ✗ | ✓ |
| JIT Access | ✗ | ✓ |

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    GATEWISE CONTROL PLANE                   │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   CLI       │  │   REST API  │  │  Dashboard  │         │
│  │ (gatewise)  │  │   (Future)  │  │  (Future)   │         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          ▼                                  │
│  ┌───────────────────────────────────────────────────────┐ │
│  │              POLICY ENGINE                            │ │
│  │  ┌─────────┐  ┌───────────┐  ┌──────────────┐        │ │
│  │  │ Parser  │→ │ Validator │→ │ Reconciler   │        │ │
│  │  │ (YAML)  │  │           │  │ (client-go)  │        │ │
│  │  └─────────┘  └───────────┘  └──────────────┘        │ │
│  └───────────────────────────────────────────────────────┘ │
│                          │                                  │
│                          ▼                                  │
│  ┌───────────────────────────────────────────────────────┐ │
│  │           CONNECTOR FACTORY (Pattern Factory)         │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │ │
│  │  │ K8sConnector│  │KafkaConnect│  │ KongConnect │  │ │
│  │  │  (Embedded) │  │ (External) │  │ (External)  │  │ │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  │ │
│  └───────────────────────────────────────────────────────┘ │
│                          │                                  │
│                          ▼                                  │
│  ┌───────────────────────────────────────────────────────┐ │
│  │                  PostgreSQL                           │ │
│  │   • Policies • Audit Logs • JIT Grants               │ │
│  └───────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## 🚀 Installation

```bash
# Build depuis les sources
cd /app/gatewise
go build -o gatewise ./cmd/gatewise-cli/

# Vérifier la version
./gatewise --version
```

## 📝 Utilisation CLI

### Vérifier le statut
```bash
# Mode Community
./gatewise status

# Mode Enterprise
./gatewise status --license ENT-YOUR-KEY
```

### Parser une politique
```bash
./gatewise parse configs/examples/marketing-team-access.yaml
./gatewise parse configs/examples/marketing-team-access.yaml -a   # Multi-documents
./gatewise parse configs/examples/marketing-team-access.yaml -o json
```

### Valider une politique
```bash
./gatewise validate configs/examples/marketing-team-access.yaml
```

### Appliquer une politique (dry-run)
```bash
./gatewise apply --dry-run configs/examples/marketing-team-access.yaml
```

### Appliquer une politique sur K8s
```bash
# Mode Embedded (in-cluster)
./gatewise apply configs/examples/marketing-team-access.yaml \
    --db-url "postgres://gatewise:gatewise123@localhost:5432/gatewise?sslmode=disable"

# Mode External (kubeconfig)
./gatewise apply configs/examples/marketing-team-access.yaml \
    --kubeconfig ~/.kube/config \
    --db-url "postgres://gatewise:gatewise123@localhost:5432/gatewise?sslmode=disable"
```

### Lister les politiques
```bash
./gatewise get policies --db-url "postgres://..." -o table
```

## 📋 Format de Politique YAML

```yaml
apiVersion: gatewise.io/v1alpha1
kind: AccessPolicy
metadata:
  name: my-team-access
  namespace: production
  labels:
    team: platform
spec:
  team: platform
  members:
    - user@company.com
    - group:developers        # Prefix 'group:' pour les groupes
    - sa:my-service-account   # Prefix 'sa:' pour les ServiceAccounts
  targets:
    - type: kubernetes
      name: k8s-prod
      kubernetes:
        namespace: production
        resources: [pods, services, deployments]
        verbs: [get, list, create, update, delete]
    - type: kafka
      name: kafka-events
      kafka:
        topics: [events, logs]
        operations: [READ, WRITE]
    - type: kong
      name: kong-api
      kong:
        routes: [/api/v1/*]
        services: [main-api]
  jit:
    enabled: true
    duration: "2h"
    approvalRequired: true
  quotas:
    cpu: "4"
    memory: "8Gi"
    pods: 20
```

## 📁 Structure du Projet

```
/app/gatewise/
├── cmd/
│   ├── gatewise-cli/        # CLI principal
│   └── test-store/          # Tests PostgreSQL
├── internal/
│   ├── policy/              # Parser et Validator
│   ├── store/               # PostgreSQL store
│   ├── connector/           # Connecteurs infrastructure
│   │   ├── connector.go     # Interface Connector
│   │   ├── factory.go       # Pattern Factory
│   │   └── kubernetes.go    # K8s Connector (client-go)
│   └── reconciler/          # Boucle de réconciliation
├── pkg/
│   ├── models/              # Structures de données
│   └── license/             # Gestion des licences
├── configs/
│   └── examples/            # Exemples de politiques
├── go.mod
└── README.md
```

## 🎛️ Kubernetes Connector

Le connecteur Kubernetes utilise `client-go` pour créer automatiquement :

| Ressource | Description |
|-----------|-------------|
| Namespace | Isolé avec labels Gatewise |
| Role | Permissions RBAC par namespace |
| RoleBinding | Liaison utilisateurs/groupes → rôle |
| ResourceQuota | Limites CPU/mémoire/pods |

### Labels automatiques
```yaml
gatewise.io/managed: "true"
gatewise.io/policy: "policy-name"
gatewise.io/team: "team-name"
```

## 🛣️ Roadmap

| Phase | Objectif | État |
|-------|----------|------|
| 1 | Modèle & Parser YAML | ✅ Fait |
| 2 | Validation Engine | ✅ Fait |
| 3 | PostgreSQL Store | ✅ Fait |
| 4 | CLI Gatewise | ✅ Fait |
| 5 | K8s Connector (client-go) | ✅ Fait |
| 6 | License Manager | ✅ Fait |
| 7 | Connector Factory (BYOI) | ✅ Fait |
| 8 | Reconciler | ✅ Fait |
| 9 | Connecteur Kafka | 🔜 À venir |
| 10 | Connecteur Kong | 🔜 À venir |
| 11 | Dashboard React | 🔜 À venir |
| 12 | Helm Chart | 🔜 À venir |

## 📄 License

Open Source (Open Core) - Gatewise Community est gratuit. Gatewise Enterprise débloque les connecteurs avancés.
