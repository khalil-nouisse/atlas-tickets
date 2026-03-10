# Troubleshooting

This document outlines common issues encountered when deploying or testing AtlasTickets, and how to resolve them.

## Bookings Not Appearing After a Successful k6 Run

**Symptom:** k6 reports 100% success rate but PostgreSQL shows 0 bookings.

**Diagnosis:**

```bash
# Check RabbitMQ queue depth
kubectl port-forward -n atlastickets svc/rabbitmq-service 15672:15672
# Open http://localhost:15672 (guest / guest) -> Queues -> ticket_queue

# Check query-service consumer logs for errors
kubectl logs -l app=query-service -n atlastickets --tail=100

# Check command-service logs for publish failures
kubectl logs -l component=command-service -n atlastickets --tail=100
```

**Common cause:** The `ticket_queue` exists but the consumer is not connected. Restart the query service:

```bash
kubectl rollout restart deployment query-service -n atlastickets
```

---

## Prometheus Shows query-service as DOWN

**Symptom:** Status > Targets in Prometheus shows `query-service` with state `DOWN` or `Error scraping target: 404`.

**Cause:** Pod is running an image that predates the `/metrics` route implementation.

**Fix:**

```bash
# Force a new rollout to pull the latest image
kubectl rollout restart deployment query-service -n atlastickets
kubectl rollout status deployment query-service -n atlastickets

# Verify the metrics endpoint is accessible
kubectl port-forward -n atlastickets svc/query-service 4000:4000
curl http://localhost:4000/metrics | grep atlastickets
```

---

## Ingress Returns 404 on Cloud (K3s)

**Symptom:** `curl http://api.localhost/api/tickets` returns 404 when the cluster is K3s.

**Cause:** The `kind/` ingress is NGINX-based and uses `api.localhost` as the host rule. K3s uses Traefik with nip.io hostnames.

**Fix:** Use the cloud hostname:

```bash
# Correct (K3s / cloud)
curl http://api.<MASTER_PUBLIC_IP>.nip.io/api/tickets

# Correct (Kind / local)
curl http://api.localhost/api/tickets
```

---

## postgres-0 Pod Stuck in Pending

**Symptom:** `kubectl get pods -n atlastickets` shows `postgres-0` as `Pending`.

**Diagnosis:**

```bash
kubectl describe pod postgres-0 -n atlastickets
# Look for: "no persistent volume available for this claim"
```

**Fix:** Ensure StorageClass is available on the node and apply the PVC:

```bash
kubectl get storageclass
kubectl get pvc -n atlastickets
```

On K3s, the default `local-path` StorageClass is pre-installed. On Kind, ensure the PVC spec does not request a StorageClass that does not exist in the cluster.

---

## Resetting the Database Between Test Runs

When performing multiple load tests, you must reset the state of all three databases simultaneously.

```bash
# Reset PostgreSQL bookings, MongoDB bookings, and Redis cache
./scripts/reset-database.sh

# Verify clean state
kubectl exec -it postgres-0 -n atlastickets -- psql -U atlas -d atlastickets -c \
  "SELECT match_id, category, total_seats, sold_seats FROM ticket_inventory;"

kubectl exec -it mongo-0 -n atlastickets -- mongosh tickets_read_db --quiet --eval \
  "print('MongoDB bookings:', db.bookings.countDocuments());"
```
