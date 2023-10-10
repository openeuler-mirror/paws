#!/bin/bash
#deploy test vpa
#script to start all the components of predictive VPA.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
echo "Deploying Redis VPA object"
kubectl apply -f $SCRIPT_DIR/../manifests/workloads/redis/redis-workload-deployment.yaml
status=$?
[ $status -eq 0 ] && echo "Deployed successfully" || echo "Deployment failed.. `exit`"
