# Gatewise - Product Requirements Document

## Projet
**Gatewise** - The Unified Access Layer for Modern Infrastructure

## Date de création
12 Mars 2026

## Vision Enterprise
Gatewise orchestre l'accès à Kubernetes, Kafka et Kong depuis un seul Control Plane avec:
- **BYOI (Bring Your Own Infrastructure)** : Mode Embedded ou External
- **Tiering Licences** : Community (K8s de base) vs Enterprise (Kafka, Kong, Keycloak, Cerbos)
- **Pattern Factory** : Architecture extensible pour les connecteurs

---

## Ce qui a été implémenté

### Phase 1 - Core Engine (Go)
- ✅ Parser YAML multi-documents
- ✅ Validation Engine complet
- ✅ PostgreSQL Store (policies, audit_logs, jit_grants)
- ✅ CLI Gatewise (parse, validate, apply, status, get, reconcile)

### Phase 2 - Connecteurs (Go + client-go)
- ✅ K8s Connector : Namespace, Role, RoleBinding, ResourceQuota
- ✅ Kafka Connector : Topics, ACLs via Sarama SDK
- ✅ Kong Connector : Services, Routes, Plugins via Admin API
- ✅ License Manager : Community vs Enterprise tiering
- ✅ Connector Factory : Pattern BYOI

### Phase 3 - Control Plane API & Dashboard
- ✅ REST API (Python FastAPI wrapper pour compatibilité Emergent)
  - GET /api/health, /api/status
  - CRUD /api/policies
  - POST /api/policies/validate, /api/policies/:name/reconcile
  - GET/POST /api/license
  - GET /api/connectors
- ✅ React Dashboard
  - Dashboard avec stats et statut connecteurs
  - Page Policies avec CRUD et validation YAML
  - Page Connectors (K8s Community, Kafka/Kong Enterprise)
  - Page Settings pour gestion licence
  - Page Audit Logs

---

## Stack Technique

| Composant | Technologie |
|-----------|-------------|
| Backend Go | Go 1.19, client-go, Sarama, Cobra |
| Backend API | Python FastAPI (wrapper) |
| Frontend | React 18, CSS Variables |
| Base de données | PostgreSQL 15 |
| Connecteurs | K8s client-go, Kafka Sarama, Kong Admin API |

---

## Tests Validés
- Backend API: 13/13 endpoints (100%)
- Frontend: Toutes les pages fonctionnelles (100%)
- Go CLI: 14/14 commandes (100%)

---

## Backlog Priorisé

### P0 (Complété ✅)
- [x] Connecteur Kafka (Sarama SDK)
- [x] Connecteur Kong (Admin API)
- [x] API REST Control Plane
- [x] Dashboard React

### P1 (À venir)
- [ ] Self-Healing actif (Anti-Drift detection)
- [ ] JIT Access avec expiration auto
- [ ] Webhooks pour notifications

### P2 (Future)
- [ ] Helm Chart intelligent
- [ ] Keycloak SSO Integration
- [ ] Cerbos Authz
- [ ] Multi-cluster support

---

## Prochaines Actions Suggérées
1. Déployer sur un cluster K8s réel pour tests end-to-end
2. Implémenter Self-Healing avec watcher K8s
3. Ajouter websockets pour updates temps réel
4. Créer Helm Chart avec values.yaml configurables
