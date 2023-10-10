#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
echo "Deleting SIR vpa recommender"
kubectl delete -f $SCRIPT_DIR/../manifests/core/sir-recommender-deployment.yaml
kubectl delete -f $SCRIPT_DIR/../manifests/vpa_objects/redis_vpa.yaml
kubectl delete -f $SCRIPT_DIR/../manifests/priority-classes/
status=$?
[ $status -eq 0 ] && echo "Deployment Deleted" || echo "Deployment failed to delete.. `exit`"

