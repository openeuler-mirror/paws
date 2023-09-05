# Paws: Performance Aware System

A large-scale cluster is made up of many heterogenous physical machines, i.e. different hardware architecture or different capacity such as CPU cores, memory available and accelerators.

Resource utilization metrics such as CPU utilization % or GPU utilization % are leveraged to quantify how well a cluster is used *effectively* by the users. Resource utilization can be affected by the resources *assigned* to the workload i.e. how much resources are assigned, as well as the *placement* of the workload, i.e. where the workload is placed.

If the workload resource assignment is inaccurate, the cluster can experience underutilization due to *idleness*. On the other hand, if the workload placement is suboptimal, the workload can experience performance degradation due to interference between workload, i.e. contending for resources.

Thus, the overall objective of this project is to *improve resource utilization (currently only **CPU**) while minimizing the performance degradation due to interference*, to avoid violating the Quality of Service (QoS) guarantee. In the following section, we present the placement challenge and follow by the assignment challenge.

## Placement - Interference

The interference effects can be caused by physical hardware or software implementation. A few common causes of the physical hardware interference can be:

1. Cache Misses (CPU L1/L2/L3/LLC/NUMA Caches)
2. Waiting for CPU core time slices
3. Waiting for the Disk IO
4. Network bandwidth saturation

To mitigate performance interference due to hardware resource contention, researchers propose two major category of approaches to scheduling under two assumptions (i) *whitebox* approach and (ii) *blackbox* approach.

### Whitebox Scheduling Approach

Whitebox workload is defined as workload that is *known* at arrival time, i.e. the underlying workload (binary, container image), the *required inputs* to the workload, and its application performance at runtime.

Under the assumption of whitebox, researchers leverage online isolation profiling before actual execution to obtain its resource consumption characteristics to better colocate workload to avoid resource contention.

However, this is only practical when the cluster framework and the workload are both extensible, internal, visible, and allow modification from the cluster framework.

### Blackbox Scheduling Approach

Blackbox workload is defined as workload that is *unknown* at arrival time, i.e. we can only obtain the application name, the desire number of replicas, resources needed, and (maybe) the container image.

Under the assumption of blackbox, researchers leverage proxy metrics (e.g. CPU Utilization) to collocate workload to improve resource utilization.

This approach is usually more appealing to the cluster provider and the users, where limited information are shared.

However, CPU utilization is not *always* a suitable proxy metric for scheduling and large number of research shows that the inclusion of hardware metrics can provide better indication of *resource saturation*.

We postulate the question of whether we can obtain further information from the container image (if possible), and/or based on readily available system metrics, determine a better placement decision.

Related to the metrics, data-driven cluster scheduling usually makes decision in centralized fashion but based on *delayed* metrics. We investigate whether a collaborative approach can improve the placement decision.

## Assignment - Reduce Idleness

The resource underutilization challenge is not new and exists in both whitebox and blackbox scenario.

Under the assumption of whitebox, the cluster framework can tune the resource requests by monitoring the application performance metrics to avoid violating QoS.

However, in the blackbox assumption, the cluster framework is not able to obtain the application performance metrics of the workload under tuning and thus, problem remains how to best tune the resource requests without introducing resource starvation on the workload based on proxy metrics.

We postulate the question of whether we can do better than existing performance-agnostic algorithm.

## Scope

In this project, we focus on the blackbox scenario, where we can only obtain limited information about the workload, and the machine metrics.
