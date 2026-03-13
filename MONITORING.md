# Gatewise Monitoring avec Prometheus & Grafana

## 📊 Métriques Exposées

Gatewise expose des métriques Prometheus sur l'endpoint `/metrics` (port 8001 par défaut).

### Métriques Disponibles

| Métrique | Type | Description |
|----------|------|-------------|
| `gatewise_policies_total` | Gauge | Nombre total de policies actives |
| `gatewise_jit_grants_active` | Gauge | Nombre de grants JIT actifs |
| `gatewise_drifts_detected` | Gauge | Nombre de drifts détectés |
| `gatewise_api_requests_total` | Counter | Total des requêtes API |
| `gatewise_license_tier{tier}` | Gauge | Tier de licence (community/enterprise) |
| `gatewise_database_connected` | Gauge | Statut de connexion BDD (1=connecté, 0=déconnecté) |
| `gatewise_info{version}` | Gauge | Information de version |

### Limites Community Edition (visibles dans les métriques)

- **Policies** : Max 5 (métrique `gatewise_policies_total` ≤ 5)
- **JIT Grants** : Max 2 par jour (métrique `gatewise_jit_grants_active` ≤ 2)
- **Kafka Topics** : Max 3
- **Kong Routes** : Max 5
- **Clusters K8s** : 1 seul

---

## 🚀 Configuration Prometheus

### 1. Configuration manuelle

Ajoutez ce job à votre `prometheus.yml` :

```yaml
scrape_configs:
  - job_name: 'gatewise'
    static_configs:
      - targets: ['gatewise-control-plane:8001']
    metrics_path: '/metrics'
    scrape_interval: 30s
```

### 2. Via Helm Chart (Prometheus Operator)

Le Helm chart de Gatewise inclut un ServiceMonitor automatique :

```yaml
# values.yaml
monitoring:
  prometheus:
    enabled: true
    port: 9090
    path: /metrics
  
  serviceMonitor:
    enabled: true  # Active le ServiceMonitor pour Prometheus Operator
    interval: 30s
    labels:
      release: prometheus  # Adapter selon votre installation
```

Installez avec :

```bash
helm install gatewise ./helm/gatewise \
  --set monitoring.serviceMonitor.enabled=true
```

---

## 📈 Dashboard Grafana

### Import du Dashboard

Un dashboard Grafana prêt à l'emploi est disponible dans `helm/gatewise/dashboards/gatewise-dashboard.json`.

#### Option 1 : Import manuel

1. Ouvrez Grafana UI
2. Allez dans **Dashboards** → **Import**
3. Uploadez le fichier `dashboards/gatewise-dashboard.json`
4. Sélectionnez votre datasource Prometheus
5. Cliquez sur **Import**

#### Option 2 : Via Helm Chart

Le dashboard est automatiquement créé si vous activez Grafana dans le chart :

```yaml
# values.yaml
monitoring:
  grafana:
    enabled: true
    dashboardConfigMap: gatewise-grafana-dashboard
```

### Contenu du Dashboard

Le dashboard inclut :

- **Métriques Overview** : Graphiques temps réel (policies, JIT, drifts)
- **License Tier** : Jauge indiquant Community ou Enterprise
- **API Request Rate** : Taux de requêtes API par seconde
- **Database Status** : État de la connexion BDD
- **Policy Count** : Nombre de policies (avec seuil Community à 5)
- **Configuration Drifts** : Nombre de drifts détectés
- **Active JIT Grants** : Grants actifs (avec limite Community à 2/jour)

---

## 🔔 Alertes Recommandées

### Alertes Prometheus

Créez un fichier `gatewise-alerts.yaml` :

```yaml
groups:
  - name: gatewise
    interval: 1m
    rules:
      # Alerte si la BDD est déconnectée
      - alert: GatewiseDatabaseDown
        expr: gatewise_database_connected == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Gatewise database disconnected"
          description: "Database has been disconnected for more than 2 minutes"
      
      # Alerte si trop de drifts
      - alert: GatewiseHighDriftCount
        expr: gatewise_drifts_detected > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High number of configuration drifts"
          description: "{{ $value }} drifts detected, investigate manual changes"
      
      # Alerte Community limite policies
      - alert: GatewiseCommunityPolicyLimit
        expr: gatewise_policies_total >= 5 AND gatewise_license_tier{tier="community"} == 1
        for: 1m
        labels:
          severity: info
        annotations:
          summary: "Community edition policy limit reached"
          description: "Reached max 5 policies for Community tier. Upgrade to Enterprise for unlimited."
```

Chargez les alertes :

```bash
kubectl apply -f gatewise-alerts.yaml
```

---

## 🧪 Tester les Métriques

### Test local

```bash
# Vérifier l'endpoint metrics
curl http://localhost:8001/metrics

# Avec Prometheus Query
curl -g 'http://localhost:9090/api/v1/query?query=gatewise_policies_total'
```

### Créer des données de test

```bash
# Créer une policy (augmente gatewise_policies_total)
curl -X POST http://localhost:8001/api/policies \
  -H "Content-Type: application/json" \
  -d '{"yaml": "..."}'

# Demander un JIT grant (augmente gatewise_jit_grants_active)
curl -X POST http://localhost:8001/api/jit/request \
  -H "Content-Type: application/json" \
  -d '{"policyName": "test", "grantee": "user@example.com"}'
```

---

## 📦 Déploiement Complet (Prometheus + Grafana + Gatewise)

### Via Helm

```bash
# 1. Installer kube-prometheus-stack (Prometheus + Grafana)
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace

# 2. Installer Gatewise avec monitoring activé
helm install gatewise ./helm/gatewise \
  --set monitoring.serviceMonitor.enabled=true \
  --set monitoring.serviceMonitor.labels.release=prometheus \
  --set monitoring.grafana.enabled=true

# 3. Accéder à Grafana
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80

# Login: admin / prom-operator (mot de passe par défaut)
```

---

## 🔍 Troubleshooting

### Les métriques n'apparaissent pas dans Prometheus

1. Vérifiez que l'endpoint est accessible :
   ```bash
   kubectl exec -it <gatewise-pod> -- curl localhost:8001/metrics
   ```

2. Vérifiez les targets Prometheus :
   - Ouvrez Prometheus UI : `http://localhost:9090/targets`
   - Cherchez le job `gatewise`
   - Statut doit être **UP**

3. Vérifiez les labels du ServiceMonitor :
   ```bash
   kubectl get servicemonitor -n gatewise -o yaml
   ```

### Le dashboard Grafana est vide

1. Vérifiez la datasource Prometheus dans Grafana
2. Testez une query simple : `gatewise_info`
3. Vérifiez que Prometheus scrape bien Gatewise (voir ci-dessus)

---

## 📚 Ressources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Dashboards](https://grafana.com/docs/grafana/latest/dashboards/)
- [Kubernetes Monitoring](https://kubernetes.io/docs/tasks/debug/debug-cluster/resource-metrics-pipeline/)
