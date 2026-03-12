# Gatewise - Product Requirements Document

## Projet
**Gatewise** - The Unified Access Layer for Modern Infrastructure

## Date de création
12 Mars 2026

## Problem Statement Original
Gatewise est une plateforme Open Source (Open Core) qui permet de gérer les accès et la sécurité des infrastructures modernes de manière centralisée. Au lieu de configurer chaque outil manuellement, l'administrateur définit une intention (ex: "L'équipe Marketing a accès à K8s, Kafka et l'API Ads") et Gatewise s'occupe de tout.

## Architecture
- **Control Plane (Serveur)** : Cerveau du système - API/CLI, PostgreSQL, Audit
- **Agent In-Cluster** : Bras armé - Pull des politiques, application locale

## Stack Technique Choisie
- Backend: Go 1.19
- Base de données: PostgreSQL 15
- CLI: Cobra framework
- Frontend: React (à venir)

## Utilisateurs Cibles
- Administrateurs infrastructure
- DevOps / SRE
- Platform Engineers
- Security Teams

## Core Requirements (Statique)
1. Parsing YAML des politiques
2. Validation complète des politiques
3. Stockage PostgreSQL
4. CLI fonctionnel
5. Support K8s, Kafka, Kong
6. Just-In-Time Access
7. Self-Healing (Anti-Drift)
8. Audit unifié

---

## Ce qui a été implémenté (Phase 1 - 12 Mars 2026)

### ✅ Étape 1 : Modèle & Parser YAML
- Structure Go complète pour les politiques (`pkg/models/policy.go`)
- 3 types de Kind : AccessPolicy, TeamBinding, ResourceQuota
- Support targets : Kubernetes, Kafka, Kong
- Configuration JIT intégrée
- Parser YAML avec support multi-documents

### ✅ Étape 2 : Validation Engine
- Validation API Version (gatewise.io/v1alpha1, v1beta1, v1)
- Validation Kind
- Validation DNS-1123 pour les noms
- Détection des noms interdits (kube-system, default, etc.)
- Validation spécifique par type de cible
- Validation JIT duration format

### ✅ Étape 3 : PostgreSQL Store
- Table `policies` avec JSONB pour spec
- Table `audit_logs` pour l'historique
- Table `jit_grants` pour l'accès temporaire
- CRUD complet testé

### ✅ Étape 4 : CLI Gatewise
- `gatewise parse` - Affichage politique (YAML/JSON/table)
- `gatewise parse -a` - Parsing multi-documents
- `gatewise validate` - Validation complète
- `gatewise apply` - Préparé (validation incluse)
- `gatewise get` - Préparé

---

## Backlog Priorisé

### P0 (Critique - Prochaine étape)
- [ ] Agent Reconciler Kubernetes (client-go)
- [ ] Boucle de réconciliation

### P1 (Important)
- [ ] Connecteur Kafka (ACLs)
- [ ] Connecteur Kong (Routes, Services, Plugins)
- [ ] API REST Control Plane

### P2 (Nice to have)
- [ ] Dashboard React
- [ ] Self-Healing / Anti-Drift
- [ ] Just-In-Time Access avec auto-expiration
- [ ] Capability Discovery
- [ ] Helm Chart

---

## Prochaines Actions
1. Implémenter le connecteur Kubernetes avec client-go
2. Coder la boucle de réconciliation (watch + apply)
3. Ajouter le connecteur Kafka
4. Ajouter le connecteur Kong
5. Créer l'API REST pour le Control Plane
6. Dashboard React
