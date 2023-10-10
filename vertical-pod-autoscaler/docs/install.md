1. [Installation](#installation)
2. [Prerequisite](#prerequisites)
3. [Quick Installation](#quick-installation-and-testing-using-scripts-in-PAWS-k8s-cluster)
4. [Detailed Installation](#detailed-installation-step-by-step-with-configuration-options)
5. [Priority Classes](#priority-classes)
6. [Testing](#testing-with-custom-vpa-object-and-redis-workload)
7. [Run Book](#run-book)
   1. [504 Error](#504-error)

## Prerequisites
- Kubernetes 1.22+ cluster or Kind Cluster
- Kubeconfig to connect to the Kubernetes cluster
- [Kubernetes Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)
- A docker repository

## Quick Installation and Testing using Scripts in PAWS K8S cluster
Helper scripts are provided for quick and simplified installation

```bash
#Install VPA 
git clone https://github.com/kubernetes/autoscaler.git
cd vertical-pod-autoscaler
./hack/vpa-up.sh

git clone https://gitee.com/openeuler/paws.git
cd vertical-pod-autoscaler/deploy_scripts
# To deploy PAWS vpa recommender execute the script below
./start.sh

#Deploy VPA Object (Redis), deployed in the default workspace
./deploy_test_vpa_object.sh


#Deploy test workload (Redis)
./deploy_test_workload.sh

#Check if VPA installed and check if it is working
kubectl get vpa
kubectl get pods -n kube-system
kubectl logs [vpa-recommender_pod_name] -n kube-system --follow

# To teardown PAWS vpa recommender execute the script below
./delete.sh
```

## Detailed Installation Step by Step with configuration options

0. Install VPA in Kubernetes
We need the updater and admission controller components of K8S VPA for the recommender to work.

```bash
git clone https://github.com/kubernetes/autoscaler.git
cd vertical-pod-autoscaler
./hack/vpa-up.sh
```
For more details visit or troubleshooting visit [VPA Repository](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)

1. Install PAWS recommender
2. Update the prometheus settings (PROM_URL) in [here](./recommender_config.yaml) and [here](./manifests/core/recommender-deployment.yaml) to match your local Kubernetes settings
3. Push to an accessible docker registry, some helper commands below.

```bash
git clone https://gitee.com/openeuler/paws.git
cd vertical-pod-autoscaler
tag=v1.0.0

#Change your repo name accordingly
docker build -t docker.io/repo/paws-recommender:$tag .
docker push docker.io/repo/paws-recommender:$tag
```

3. Install PAWS recommender, clone if not already cloned in step one. This will install the PAWS VPA recommender in the cluster.
```bash
git clone https://gitee.com/openeuler/paws.git

cd vertical-pod-autoscaler
kubectl apply -f manifests/core/recommender-deployment.yaml

#Check if the recommender is deployed using the command below
kubectl get deployments -n kube-system | grep "recommender name"
```
## Priority Classes
PAWS VPA uses priority classes to select the best recommender algorithm for a particular workload. Low priority workload 
can tolerate higher throttling compared to higher priority workloads. You can define your own pod priority classes. See 
[Kubernetes Pod Priority](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/)

4. Install Priority Classes

```bash
cd priority-classes

#Install the priority classes
kubectl apply -f priority-classes/
```

## Testing with custom VPA object and Redis workload

Now that the installation of the VPA and the custom recommender is completed, the next step is to deploy a workload to be controlled the VPA ecosystem.
There are two steps to accomplish. Deploy VPA object that point to a specific workload, this workload can be any kubernetes object e.g. Deployment, StatefulSet etc.

5. Deploy VPA CRD to monitor workloads or pods
```bash 
kubectl apply -f manifests/vpa_objects/redis_vpa.yaml

#To check if VPA object is successfully installed execute
kubectl get vpa
```

6. Deploy the workload. There are some definitions to add redis metrics exporter to the cluster. If this is not needed just use only the redis deployment
```bash
kubectl apply -f manifests/workloads/redis/redis-workload-deployment.yaml
```

7. Check if the VPA is installed
kubectl get vpa
kubectl get pods -n kube-system

8.  Check if PAWS recommender is running, if so you should see some stats as below
```bash  
#Run the command below and note the pod name
kubectl get deployments -n kube-system

#Check the logs
kubectl -n kube-system logs [paws-recommender-pod-name] --follow
```

```bash 
Successfully patched VPA object with the recommendation: 
    [
      {
        'containerName': 'the-container-name', 'lowerBound': {'cpu': '100m', 'memory': '50Mi'}, 
        'target': {'cpu': '100m', 'memory': '50Mi'}, 
        'uncappedTarget': {'cpu': '10m', 'memory': '8Mi'}, 
        'upperBound': {'cpu': '100m', 'memory': '50Mi'}
      }
    ]
 ```

9. Check default VPA recommender, the count of VPA objects should be 0
```bash
kubectl logs [vpa-recommender_pod_name] -n kube-system --follow
```


# Run Book
## 504 error
This is the case when the driver cannot access prometheus, there are two cases in which prometheus can be accessed. .

1. Prometheus running as a service within the cluster, this is ideal and it is used when in production or if you are not testing.
This is the case when yaml files are deployed as described in the guide above.
2. The other option is when in development. This is when Prometheus is accessed from outside the cluster through port forwarding.

The configuration are as to which prometheus access method the recommender is going to use is specified in recommender_config.yaml file, 
the processing is done in recommender_config.py