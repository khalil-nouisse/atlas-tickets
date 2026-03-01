#!/bin/bash
set -e

echo "Deploying to Oracle Cloud K3s..."

# Switch to Oracle context
export KUBECONFIG=~/.kube/config-oracle

# Deploy base
kubectl apply -f k8s/base/

# Deploy infrastructure
kubectl apply -f k8s/infrastructure/postgres/
kubectl apply -f k8s/infrastructure/mongodb/
kubectl apply -f k8s/infrastructure/redis/
kubectl apply -f k8s/infrastructure/rabbitmq/

# Wait for infrastructure
kubectl wait --for=condition=ready pod -l app=postgres -n atlastickets --timeout=300s
kubectl wait --for=condition=ready pod -l app=mongo -n atlastickets --timeout=300s

# Initialize DB
kubectl apply -f k8s/jobs/db-init-job.yaml

# Deploy services
kubectl apply -f k8s/services/command-service
kubectl apply -f k8s/services/query-service

# Deploy Traefik ingress
kubectl apply -f k8s/infrastructure/ingress/traefik/

# Get master IP
echo ""
echo "Deployed to K3s with Traefik ingress"
echo "Access at: http://api.84.8.216.45.nip.io/api/tickets"
