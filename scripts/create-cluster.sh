#!/bin/bash
set -e  # Exit immediately if any command fails


echo "Creating Kind Cluster..."
echo ""


# Delete existing cluster if it exists
echo "Deleting existing Cluster..."
kind delete cluster --name atlastickets-dev 2>/dev/null || true


echo ""
echo "Creating new Cluster"
kind create cluster --name atlastickets-dev --config ../infrastructure/kind/cluster-config.yaml


echo ""
echo "Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s


echo ""
echo "Cluster created successfully!"
echo ""
echo "Cluster information:"
kubectl cluster-info --context kind-atlastickets-dev


echo ""
echo "Nodes in the cluster:"
kubectl get nodes -o wide

echo ""
echo "Namespaces:"
kubectl get namespaces

echo ""
echo "Cluster is ready! Context is set to: kind-atlastickets-dev"
