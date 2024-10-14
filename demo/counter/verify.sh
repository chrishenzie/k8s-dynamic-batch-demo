#!/bin/bash

STARTING_COUNT=$(kubectl get taskobject input -o 'jsonpath={.data.count}')
STARTING_LIMIT=$(kubectl get taskobject input -o 'jsonpath={.data.limit}')
FINAL_COUNT=$(kubectl get taskobject task-49-output -o 'jsonpath={.data.count}')
FINAL_LIMIT=$(kubectl get taskobject task-49-output -o 'jsonpath={.data.limit}')

echo 'Initial TaskObject: "input"'
echo "count: ${STARTING_COUNT}"
echo -e "limit: ${STARTING_LIMIT}\n"

echo 'Final TaskObject: "task-49-output"'
echo "count: ${FINAL_COUNT}"
echo "limit: ${FINAL_LIMIT}"
