# Temporal Utilization

## Overview

The plugin utilizes the (CPU) utilization template extracted from historical data for a pod to achieve better 'peak shaving' and 'valley filling' effect.

<img src="../images/temporal_utilization.svg" alt="workflow_overview" width="700">

## Pre-requisites

The following components should be deployed:

- Prometheus
- cAdvisor

The following metrics are exposed from cAdvisor (must be deployed first):

- container_cpu_usage_total, this allows us to query the pods usage for a specific application

Additionally, the container_cpu_usage_total should have the container labels stored.

To achieve this, one can configure cAdvisor (only as a separate daemonset), see [here](https://github.com/kubernetes/kubernetes/issues/79702) and set the following [flags](https://github.com/google/cadvisor/blob/master/docs/runtime_options.md#container-labels):

- either, `--store_container_labels=true`
- or `--store_container_labels=false` but set the `--whitelisted_container_labels` flag to include the recommended [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/). 

## Assumptions

The plugin has the following assumptions:

- The cluster consists of recurring pods.
- Some of these pods are long running.

## Usage

1. We use a unique label key named `scheduling.x-k8s.io/usage-template` to define a specific Usage Template Evaluation Request. Pods that have the labels and have the same value are identified as belonging to the same UsageTemplateEvaluation. 

2. We expect user to label their pods with unique labels, specifically we require `container` to be set. These labels can uniquely identify the application and container to calculate the utilization values, where container is the container `name` attribute.

```yaml
# UsageTemplate CRD
apiVersion: scheduling.x-k8s.io/v1alpha1
kind: UsageTemplate
metadata:
  name: product-svc-app1
spec:
  enabled: true
  evaluatePeriodHours: 6 # evaluate the usage every 6 hours
  resources:
  - cpu # currently only support CPU
  joinLabels: # This is needed if you are using containerd and standalone cadvisor
  - part_of
  filters:
  - app.kubernetes.io/part-of=product-svc-app1
  - container=abc
  qualityOfServiceClass: Guaranteed

---
# The pod with the associate labels
labels:
  scheduling.x-k8s.io/usage-template: product-svc-app1
  app.kubernetes.io/part-of: product-svc-app1 # this label is for cadvisor to tag the container cpu usage
```


3. To maximize resoure utilization we recommend disable the default plugins i) `NodeResourcesFit` ii) `NodeResourcesBalancedAllocation`, and turn on `EnableOvercommit` in Temporal Utilization Plugin Args. During each scheduling cycle filtering phase, we look for a node `scheduling.x-k8s.io/<resource>-overcommit-ratio` **annotation** to do the filtering to enable overcommitment.

```yaml
# Your scheduler configmap
#...
plugins:
  multiPoint:
    enabled:
    #...
    disabled: 
      - NodeResourcesFit
      - NodeResourcesBalancedAllocation
  pluginConfig:
    - name: TemporalUtilization
      args:
        hotSpotThreshold: 60
        enableOvercommit: true
```

## Limitations

This feature operates on a per hour level forecast (i.e. value in the utilization template) and taking the configured percentile of the application usages. The potential limitations are:

- ignoring machine type can lead to overestimate of usages
- the percentile are based on the containers that are part of an application and hence can lead to overestimation
