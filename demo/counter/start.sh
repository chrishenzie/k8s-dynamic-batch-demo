#!/bin/bash

set -e

# Start the 0th Task (that will trigger downstream Tasks).
kubectl apply -f demo/counter/task.yaml
