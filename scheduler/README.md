# PAWS

The PAWS Scheduler comprises scheduling features (plugins) that are data-driven in order to improve workload performance and drive up cluster utilization. The PAWS Scheduler is based on the [kubernetes scheduler framework](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/).

## Plugins

The PAWS Scheduler contains the following plugins, and can be configured in the same way as the kubernetes scheduler via scheduler profiles:

- [Temporal Utilization](./docs/features/temporalutilization.md)

## Install

We provide helm charts to install the PAWS Scheduler artifacts. Currently, the helm chart provides the ability to deploy it as a [second scheduler](./manifests/install/paws-system/README.md) similar to the kubernetes [scheduler-plugin](https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/doc/install.md#as-a-second-scheduler).

