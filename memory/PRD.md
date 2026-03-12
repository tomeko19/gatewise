# Gatewise - Product Requirements Document

## Projet
**Gatewise** - The Unified Access Layer for Modern Infrastructure

## Date de création
12 Mars 2026

## Problem Statement Original
Gatewise est une plateforme Open Source (Open Core) qui permet de gérer les accès et la sécurité des infrastructures modernes de manière centralisée pour Kubernetes, Kafka, et Kong.

## Vision Enterprise
- **BYOI (Bring Your Own Infrastructure)** : Mode Embedded ou External
- **Tiering Licences** : Community (K8s de base) vs Enterprise (Kafka, Kong, Keycloak, Cerbos)
- **Pattern Factory** : Architecture extensible pour les connecteurs

## Architecture
- **Control Plane (Serveur)** : API/CLI, PostgreSQL, Audit, License Manager
- **Connector Factory** : Création dynamique des connecteurs (K8s, Kafka, Kong)
- **Reconciler** : Boucle de réconciliation des politiques

## Stack Technique
- Backend: Go 1.19 avec client-go
- Base de données: PostgreSQL 15
- CLI: Cobra framework
- Frontend: React (à venir)

---

## Ce qui a été implémenté (Phase 2 - 12 Mars 2026)

### ✅ Étape 1 : Modèle & Parser YAML
- Structure Go complète pour les politiques
- Support multi-documents YAML

### ✅ Étape 2 : Validation Engine
- Validation complète (API version, Kind, DNS-1123, JIT)

### ✅ Étape 3 : PostgreSQL Store
- Tables: policies, audit_logs, jit_grants
- CRUD complet

### ✅ Étape 4 : CLI Gatewise v0.2.0
- `gatewise parse` - YAML/JSON/table output
- `gatewise validate` - Validation complète
- `gatewise apply` - Application avec dry-run
- `gatewise status` - Statut licence et features
- `gatewise get` - Liste des politiques
- `gatewise reconcile` - Réconciliation bulk

### ✅ Étape 5 : Connecteur Kubernetes (client-go)
- Création Namespace avec labels Gatewise
- Création Role/RoleBinding RBAC
- Création ResourceQuota
- Support users, groups, ServiceAccounts
- Mode Embedded (in-cluster) et External (kubeconfig)

### ✅ Étape 6 : License Manager
- Tier Community : K8s basic, Namespace, Quota, Audit
- Tier Enterprise : Kafka, Kong, Keycloak, Cerbos, JIT, Self-Healing

### ✅ Étape 7 : Connector Factory (Pattern Factory)
- Interface Connector générique
- Factory avec validation de licence
- Support BYOI (Embedded/External)

### ✅ Étape 8 : Reconciler
- Boucle de réconciliation
- Support multi-targets
- Audit log automatique
- Drift Detector (structure)

---

## Backlog Priorisé

### P0 (Critique)
- [ ] Connecteur Kafka (Sarama SDK)
- [ ] Connecteur Kong (Admin API)

### P1 (Important)
- [ ] API REST Control Plane
- [ ] Self-Healing (Anti-Drift) actif
- [ ] JIT Access avec expiration auto

### P2 (Nice to have)
- [ ] Dashboard React
- [ ] Helm Chart intelligent
- [ ] Keycloak SSO
- [ ] Cerbos Authz

---

## Prochaines Actions
1. Implémenter connecteur Kafka avec Sarama
2. Implémenter connecteur Kong avec Admin API
3. API REST pour le Control Plane
4. Dashboard React de visualisation
5. Helm Chart avec values.yaml configurables

## Tests Validés
- CLI: 14/14 tests passés
- Parsing YAML, validation, dry-run, status, tiering licence
