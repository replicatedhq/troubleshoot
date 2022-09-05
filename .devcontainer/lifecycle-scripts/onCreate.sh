#!/usr/bin/env bash

# Setup the cluster
k3d cluster create --config /etc/replicated/k3d-cluster.yaml --kubeconfig-update-default

# Clone any extra repos here


