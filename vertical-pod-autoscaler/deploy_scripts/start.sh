#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
echo "Installing Recommender"
kubectl apply -f $SCRIPT_DIR/../manifests/core/sir-recommender-deployment.yaml
status=$?
[ $status -eq 0 ] && echo "Deployed successfully" || echo "Deployment failed.. `exit`"
echo "Installing priority classes"
kubectl apply -f $SCRIPT_DIR/../manifests/priority-classes/
[ $status -eq 0 ] && echo "Completed successfully" || echo "Deployment Failed.. `exit`"