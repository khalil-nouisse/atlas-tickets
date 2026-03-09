#!/bin/bash

echo "Deploying to Oracle Cloud K3s..."

# Switch to Oracle context
export KUBECONFIG=~/.kube/config-oracle

# Deploy base
kubectl apply -f ../k8s/base/

# Deploy infrastructure
kubectl apply -f ../k8s/infrastructure/postgres/
kubectl apply -f ../k8s/infrastructure/mongodb/
kubectl apply -f ../k8s/infrastructure/redis/
kubectl apply -f ../k8s/infrastructure/rabbitmq/

# Wait for infrastructure (non-fatal — warn and continue if timeout)
echo "Waiting for postgres to be ready..."
if ! kubectl wait --for=condition=ready pod -l app=postgres -n atlastickets --timeout=120s; then
  echo "WARNING: postgres-0 not ready within 120s — check pod events with:"
  echo "  kubectl describe pod postgres-0 -n atlastickets"
  echo "Continuing deployment anyway..."
fi

echo "Waiting for mongo to be ready..."
if ! kubectl wait --for=condition=ready pod -l app=mongo -n atlastickets --timeout=120s; then
  echo "WARNING: mongo not ready within 120s — continuing anyway..."
fi

# Initialize DB
kubectl apply -f ../k8s/jobs/db-init-job.yaml

# Deploy services (this also enforces replica count from the yaml)
kubectl apply -f ../k8s/services/command-service
kubectl apply -f ../k8s/services/query-service

# Explicitly enforce replica counts so stale cloud replicas are corrected
echo "Enforcing replica counts..."
kubectl scale deployment/command-service --replicas=3 -n atlastickets
kubectl scale deployment/query-service   --replicas=3 -n atlastickets

# Clean up any old ReplicaSets with 0 desired replicas to free up resources
echo "Cleaning up old ReplicaSets..."
kubectl get replicaset -n atlastickets -o json | \
  python3 -c "
import sys, json
data = json.load(sys.stdin)
for rs in data['items']:
    desired = rs['spec'].get('replicas', 0)
    name = rs['metadata']['name']
    if desired == 0:
        print(name)
" | xargs -r -I{} kubectl delete replicaset {} -n atlastickets && echo "Old ReplicaSets removed." || echo "No old ReplicaSets to clean."

# Deploy Traefik ingress
kubectl apply -f ../k8s/infrastructure/ingress/traefik/

# Summary
echo ""
echo "Deployed to K3s with Traefik ingress"
echo "Access at: http://api.84.8.216.45.nip.io/api/tickets"
echo ""
echo "Current pod status:"
kubectl get pods -n atlastickets
