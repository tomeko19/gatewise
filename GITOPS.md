# Gatewise GitOps Integration

## 🎯 Vue d'ensemble

Gatewise supporte GitOps via des **Custom Resource Definitions (CRDs)** Kubernetes. Au lieu d'utiliser la CLI ou l'API REST, vous pouvez gérer vos policies comme du code dans Git, et laisser ArgoCD ou Flux les déployer automatiquement.

---

## 📦 Installation

### 1. Installer les CRDs

```bash
# Appliquer la définition CRD
kubectl apply -f gatewise/helm/gatewise/crds/gatewisepolicy-crd.yaml

# Vérifier l'installation
kubectl get crd gatewisepolicies.gatewise.io
```

### 2. Déployer l'Operator

#### Option A : Via Helm

```bash
helm install gatewise ./helm/gatewise \
  --set operator.enabled=true \
  --set operator.namespace=default
```

#### Option B : Binary manuel

```bash
# Compiler l'operator
cd gatewise
go build -o bin/gatewise-operator ./cmd/gatewise-operator

# Lancer l'operator
./bin/gatewise-operator \
  --kubeconfig ~/.kube/config \
  --namespace default \
  --db-url "postgresql://user:pass@localhost:5432/gatewise"
```

---

## 📝 Créer une Policy GitOps

### Exemple de Policy CRD

Créez `my-policy.yaml` :

```yaml
apiVersion: gatewise.io/v1
kind: GatewisePolicy
metadata:
  name: my-team-access
  namespace: default
spec:
  team: engineering
  
  kubernetes:
    namespaces:
      - name: eng-prod
        quotas:
          cpu: "8"
          memory: "16Gi"
    
    rbac:
      - role: admin
        subjects:
          - kind: User
            name: john@example.com
  
  kafka:
    topics:
      - name: eng-events
        partitions: 3
        replication: 2
  
  kong:
    routes:
      - name: eng-api
        service: eng-service
        paths:
          - /api/v1/eng
```

### Appliquer la Policy

```bash
kubectl apply -f my-policy.yaml
```

L'operator détecte automatiquement la CRD et :
1. La sauvegarde dans PostgreSQL
2. La réconcilie avec Kubernetes/Kafka/Kong
3. Met à jour le status de la CRD

### Vérifier le Status

```bash
# Lister toutes les policies
kubectl get gatewisepolicies

# Voir les détails
kubectl describe gatewisepolicy my-team-access

# Format JSON
kubectl get gatewisepolicy my-team-access -o yaml
```

Output attendu :

```yaml
status:
  phase: Active
  lastReconcileTime: "2025-01-15T10:30:00Z"
  conditions:
    - type: Ready
      status: "True"
      reason: Active
      message: "Policy successfully reconciled"
  appliedResources:
    namespaces:
      - eng-prod
    topics:
      - eng-events
    routes:
      - eng-api
```

---

## 🔄 Intégration ArgoCD

### 1. Structure du Repository Git

```
my-gitops-repo/
├── policies/
│   ├── team-a-policy.yaml
│   ├── team-b-policy.yaml
│   └── prod-policy.yaml
└── argocd/
    └── gatewise-app.yaml
```

### 2. ArgoCD Application

Créez `argocd/gatewise-app.yaml` :

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: gatewise-policies
  namespace: argocd
spec:
  project: default
  
  source:
    repoURL: https://github.com/myorg/my-gitops-repo
    targetRevision: main
    path: policies
  
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=false
```

### 3. Appliquer avec ArgoCD

```bash
kubectl apply -f argocd/gatewise-app.yaml
```

Maintenant, **chaque commit dans `policies/` déclenche une réconciliation automatique** !

---

## 🌊 Intégration Flux CD

### 1. Créer un GitRepository

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: gatewise-policies
  namespace: flux-system
spec:
  interval: 1m
  url: https://github.com/myorg/my-gitops-repo
  ref:
    branch: main
```

### 2. Créer une Kustomization

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: gatewise-policies
  namespace: flux-system
spec:
  interval: 5m
  sourceRef:
    kind: GitRepository
    name: gatewise-policies
  path: ./policies
  prune: true
  wait: true
```

### 3. Appliquer avec Flux

```bash
kubectl apply -f flux-gitrepo.yaml
kubectl apply -f flux-kustomization.yaml
```

---

## 🔍 Monitoring de l'Operator

### Logs de l'Operator

```bash
# Si déployé via Helm
kubectl logs -l app=gatewise-operator -f

# Si lancé manuellement
./bin/gatewise-operator --kubeconfig ~/.kube/config
```

Output attendu :

```
════════════════════════════════════════════════════════
  🚀 Gatewise Operator v0.3.0-alpha
  GitOps-native Kubernetes Operator for Gatewise Policies
════════════════════════════════════════════════════════

📜 License: Community
📦 Watching namespace: default

✅ Connected to PostgreSQL
✅ Kubernetes connector registered
✅ Reconciler initialized

🚀 Starting Gatewise Operator...
   Watching GatewisePolicies in namespace: default
📋 Found 2 existing GatewisePolicy resources
✅ Operator started, watching for CRD changes...

➕ Policy ADDED: my-team-access
✅ Policy my-team-access reconciled successfully
```

### Métriques Prometheus

L'operator expose des métriques sur `/metrics` :

```
gatewise_operator_reconciliations_total{policy="my-team-access",status="success"} 5
gatewise_operator_crd_watch_errors_total 0
gatewise_operator_policies_managed 3
```

---

## 🧪 Tests

### Test 1 : Appliquer une Policy

```bash
kubectl apply -f configs/examples/crd-example.yaml

# Attendre la réconciliation
kubectl wait --for=condition=Ready gatewisepolicy/marketing-team-access --timeout=60s

# Vérifier
kubectl get gatewisepolicy marketing-team-access -o jsonpath='{.status.phase}'
# Output: Active
```

### Test 2 : Modifier une Policy

```bash
# Modifier le YAML
kubectl edit gatewisepolicy marketing-team-access

# L'operator détecte le changement et réconcilie automatiquement
```

### Test 3 : Supprimer une Policy

```bash
kubectl delete gatewisepolicy marketing-team-access

# Les ressources Kubernetes/Kafka/Kong sont nettoyées automatiquement
```

---

## 🔧 Configuration Avancée

### Namespace Scoped vs Cluster Scoped

Par défaut, l'operator watch un seul namespace. Pour watch tous les namespaces :

```bash
./bin/gatewise-operator --namespace="" --kubeconfig ~/.kube/config
```

Ou via Helm :

```yaml
operator:
  watchAllNamespaces: true
```

### Limites Community Edition

Les limites s'appliquent également aux CRDs :

| Limite | Community | Enterprise |
|--------|-----------|------------|
| Policies (CRDs) | 5 max | ∞ |
| Clusters watched | 1 | ∞ |
| Kafka topics par policy | 3 max | ∞ |
| Kong routes par policy | 20 max | ∞ |

Si vous dépassez les limites, le status de la CRD sera `Failed` avec un message explicite.

---

## 🐛 Troubleshooting

### CRD non détectée

```bash
# Vérifier que le CRD est installé
kubectl get crd gatewisepolicies.gatewise.io

# Réinstaller si nécessaire
kubectl apply -f gatewise/helm/gatewise/crds/gatewisepolicy-crd.yaml
```

### Operator ne démarre pas

```bash
# Vérifier les permissions RBAC
kubectl get serviceaccount gatewise-operator
kubectl get clusterrole gatewise-operator
kubectl get clusterrolebinding gatewise-operator

# Vérifier les logs
kubectl logs -l app=gatewise-operator
```

### Policy reste en `Reconciling`

```bash
# Voir les events Kubernetes
kubectl describe gatewisepolicy <name>

# Vérifier les logs de l'operator
kubectl logs -l app=gatewise-operator | grep ERROR
```

---

## 📚 Ressources

- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [Flux CD Documentation](https://fluxcd.io/docs/)
- [Kubernetes CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [Gatewise Examples](../configs/examples/)
