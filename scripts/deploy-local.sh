#!/bin/bash
set -e

echo "eploying to kind (local)..."

# Switch to kind context
export KUBECONFIG=~/.kube/config

# Deploy base
kubectl apply -f k8s/base/

# Deploy infrastructure
kubectl apply -f k8s/infrastructure/postgres/
kubectl apply -f k8s/infrastructure/mongodb/
kubectl apply -f k8s/infrastructure/redis/
kubectl apply -f k8s/infrastructure/rabbitmq/

# Wait for infrastructure
kubectl wait --for=condition=ready pod -l app=postgres -n atlastickets --timeout=180s
kubectl wait --for=condition=ready pod -l app=mongo -n atlastickets --timeout=180s

# Initialize DB
kubectl apply -f k8s/jobs/db-init-job.yaml

# Deploy services
kubectl apply -f k8s/services/command-service
kubectl apply -f k8s/services/query-service


# Deploy NGINX ingress
kubectl apply -f k8s/infrastructure/ingress/nginx/

echo "Deployed to kind with NGINX ingress"
echo "Access at: http://api.localhost/api/tickets"
