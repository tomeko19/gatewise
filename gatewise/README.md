# Gatewise

**The Unified Access Layer for Modern Infrastructure**

Gatewise est une plateforme Open Source (Open Core) qui permet de gérer les accès et la sécurité des infrastructures modernes (Kubernetes, Kafka, Kong) de manière centralisée.

## 🎯 Vision

Au lieu de configurer chaque outil manuellement, l'administrateur définit une intention (ex: "L'équipe Marketing a accès à K8s, Kafka et l'API Ads") et Gatewise s'occupe de tout.

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
│  │  │ (YAML)  │  │           │  │ (Future)     │        │ │
│  │  └─────────┘  └───────────┘  └──────────────┘        │ │
│  └───────────────────────────────────────────────────────┘ │
│                          │                                  │
│                          ▼                                  │
│  ┌───────────────────────────────────────────────────────┐ │
│  │                  PostgreSQL                           │ │
│  │   • Policies • Audit Logs • JIT Grants               │ │
│  └───────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼ (Pull-based)
┌─────────────────────────────────────────────────────────────┐
│                  IN-CLUSTER AGENT (Future)                  │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ Kubernetes  │  │    Kafka    │  │    Kong     │         │
│  │  Connector  │  │  Connector  │  │  Connector  │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

## 🚀 Fonctionnalités (Phase 1 - MVP)

### ✅ Implémenté
- **YAML Policy Parser** : Lecture et parsing des fichiers de politique
- **Validation Engine** : Validation complète des politiques
  - Validation API Version (gatewise.io/v1alpha1, v1beta1, v1)
  - Validation Kind (AccessPolicy, TeamBinding, ResourceQuota)
  - Validation DNS-1123 pour les noms
  - Détection des noms interdits (kube-system, default, etc.)
  - Validation spécifique par type de cible (K8s, Kafka, Kong)
- **PostgreSQL Store** : Stockage persistant
  - Table `policies` pour les politiques
  - Table `audit_logs` pour l'historique
  - Table `jit_grants` pour l'accès temporaire
- **CLI Gatewise** : Interface ligne de commande
  - `gatewise parse <file>` : Parser et afficher une politique
  - `gatewise validate <file>` : Valider une politique
  - `gatewise apply <file>` : Appliquer une politique (préparé)

### 🔜 À venir
- Self-Healing (Anti-Drift)
- Just-In-Time (JIT) Access avec auto-expiration
- Auto-Détection des capacités (Capability Discovery)
- Connecteurs Kubernetes, Kafka, Kong
- Dashboard Web React

## 📦 Installation

```bash
# Build depuis les sources
cd /app/gatewise
go build -o gatewise ./cmd/gatewise-cli/

# Vérifier la version
./gatewise --version
```

## 📝 Utilisation

### Parser une politique
```bash
./gatewise parse configs/examples/marketing-team-access.yaml
./gatewise parse configs/examples/marketing-team-access.yaml -o json
```

### Valider une politique
```bash
./gatewise validate configs/examples/marketing-team-access.yaml
./gatewise validate configs/examples/jit-emergency-access.yaml
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
  targets:
    - type: kubernetes
      name: k8s-prod
      kubernetes:
        namespace: production
        resources: [pods, services]
        verbs: [get, list, create]
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
```

## 🗄️ Base de Données

### Configuration PostgreSQL
```
Host: localhost
Port: 5432
Database: gatewise
User: gatewise
Password: gatewise123
```

### Schéma
- `policies` : Stockage des politiques avec métadonnées et spec en JSONB
- `audit_logs` : Journal d'audit unifié
- `jit_grants` : Gestion des accès temporaires

## 📁 Structure du Projet

```
/app/gatewise/
├── cmd/
│   ├── gatewise-cli/    # CLI principal
│   └── test-store/      # Tests PostgreSQL
├── internal/
│   ├── policy/          # Parser et Validator
│   └── store/           # PostgreSQL store
├── pkg/
│   └── models/          # Structures de données
├── configs/
│   └── examples/        # Exemples de politiques
├── go.mod
└── README.md
```

## 🛣️ Roadmap

| Phase | Objectif | État |
|-------|----------|------|
| 1 | Modèle & Parser YAML | ✅ Fait |
| 2 | Validation Engine | ✅ Fait |
| 3 | PostgreSQL Store | ✅ Fait |
| 4 | CLI Gatewise | ✅ Fait |
| 5 | Agent Reconciler K8s | 🔜 À venir |
| 6 | Connecteur Kafka | 🔜 À venir |
| 7 | Connecteur Kong | 🔜 À venir |
| 8 | Dashboard React | 🔜 À venir |

## 📄 License

Open Source (Open Core)
