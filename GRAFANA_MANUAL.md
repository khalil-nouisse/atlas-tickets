# Prometheus & Grafana Observability Manual

## What Was Done (Monitoring Stack)

We have deployed the **kube-prometheus-stack** to the `monitoring` namespace. This includes:
-   **Prometheus**: Metrics collection and storage.
-   **Grafana**: Visualization dashboards.
-   **Alertmanager**: Alert handling.
-   **Node Exporter**: Cluster node metrics.
-   **Kube State Metrics**: Kubernetes object metrics.

---

##  User Manual: How to Access Grafana

### 1. Retrieve Admin Password
The default user is `admin`. The password is stored in a Kubernetes secret.

**Command to get password:**
```bash
kubectl get secret --namespace monitoring prometheus-grafana -o jsonpath="{.data.admin-password}" | base64 -d ; echo
```

**Current Password (for your reference):**
`DvkKTs5rNJxXS3LNj1J6VVdyyBx5BDiOU7Eb1aQQ`

### 2. Access the Dashboard
Port-forward the Grafana service to your local machine.

**Command:**
```bash
kubectl port-forward svc/prometheus-grafana 3000:80 -n monitoring
```

**URL:**
[http://localhost:3000](http://localhost:3000)

### 3. Verify Setup
1.  Go to **Dashboards** (on the left sidebar).
2.  Browse **General** folder.
3.  Open the **"Kubernetes / Compute Resources / Node (Pods)"** dashboard.
4.  You should see real-time CPU/Memory usage metrics for your cluster nodes!

---

##  Installation Guide (Reproducibility)

If you need to reinstall the stack from scratch:

```bash
# 1. Add Helm Repository
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# 2. Create Namespace
kubectl create namespace monitoring

# 3. Install Stack
helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring
```

##  Troubleshooting

**Pods remain in `ContainerCreating`?**
Check if your cluster has enough resources/storage.
```bash
kubectl describe pod -n monitoring -l app.kubernetes.io/name=prometheus
```

**Can't login?**
Double-check you decoded the password correctly (without strict mode for base64).
