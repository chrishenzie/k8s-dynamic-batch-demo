#!/bin/bash

set -e

if ! which kind &> /dev/null; then
  echo "kind is not installed or not on the PATH."
  echo "Please refer to the official docs for installation instructions:"
  echo "https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
  exit 1
fi

header() {
  echo "============================================="
  echo " ${1}"
  echo "============================================="
}

header "Step 1) Creating images"
make build-task-controller-image
make build-counter-image

header "Step 2) Creating cluster"
kind delete cluster
kind create cluster

header "Step 3) Loading images into cluster"
kind load docker-image task-controller:latest
kind load docker-image counter:latest

header "Step 4) Installing CRDs"
kubectl apply -f config/crd/taskcluster.yaml
kubectl apply -f config/crd/taskobject.yaml
kubectl apply -f config/crd/task.yaml

header "Step 5) Creating TaskCluster"
kubectl apply -f demo/counter/serviceaccount.yaml
kubectl apply -f demo/counter/taskcluster.yaml

header "Step 6) Creating task-controller Deployment"
kubectl apply -f config/controller/serviceaccount.yaml
kubectl apply -f config/controller/deployment.yaml

# Alternatively, you can run the controller outside of the cluster like so:
# make build-task-controller
# bin/task-controller -kubeconfig="${HOME}/.kube/config"
