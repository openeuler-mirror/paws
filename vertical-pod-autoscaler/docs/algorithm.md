# Vertical Pod Autoscaler Recommendation Algorithm

## Introduction

Vertical Pod Autoscaler (VPA) is a type of autoscaling technique that caters to the changing demand of a microservice by rightsizing the physical resources (CPU, memory, etc.) allocated to the microservice. Different services have different resource demands depending upon multiple factors such as time of the day, user demand, etc. Having a fixed resource allocation for these services leads to a very low resource utilization at the datacenter level.

Our Vertical Pod Autoscaler (VPA) solves this problem by adopting a hybrid approach combining the classical numerical optimization solution with the contemporary machine learning approaches. Given a workload's past runtime characteristics, our algorithm recommends the appropriate size of the resources to be allocated for the workload, thus leading to freeing up of the unused but reserved resources, thereby increase cluster utilization. Our proposed approach has the following key features: 
* **proactive** in the sense that it forecasts future recommendations and applies them prior to changes in the input workload, 
* **workload aware** as it exploits patterns in historical resource utilization to inform the recommendation decision making, and 
* **feedback mechanism** in the form of runtime throttles employed to swiftly overcome from bad recommendations. 

Note that we currently only focus on CPU resource. Moreover, we only focus on vertical scaling i.e., modifying the size (number of CPU cores) assigned to the container. This is different from Horizontal Pod Autoscaling (HPA) whose focus is to modify the number of containers allocated to a microservice, instead of size of each container. In the following subsections, we provide more technical details of our recommendation algorithm.

## VPA operating modes

We provide support for two VPA modes i.e., *initial* and *auto*.

### 1) Initial 

In *initial* mode, VPA computes the resource recommendation continuously but only applies it when a new container is being scheduled for placement.

### 2) Auto

In *auto* mode, VPA continuously provides resource recommendations after every fixed time interval. If significantly different from the current resource allocation of the container, the recommendation by VPA get applied, resulting in changing of the container size.

Note that, in general, HPA and VPA cannot be used simulatneously, if optimized under the same set of metrics. However, what makes the initial mode useful is that it can be used along with HPA without causing any conflicting issues. For further description of the two VPA operating modes, please refer [this](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).

## Optimization objective

Our algorithm takes as input the runtime execution metrics for each container. Specifically, the inputs to our algorithm are *per-minute* CPU utilization and throttle percentage timeseries for past `N` time units or minutes, called as *sample length (`N`)*. The algorithm is called continuously after every fixed interval of time, which we call *update interval (`K`)*. At each call, VPA can either (i) compute *one* optimal value of CPU resource to be allocated to each container of the deployed application, for the next update interval (i.e., `K`) minutes, or (ii) compute *multiple* values, each for one update interval in the future.

Our objective function (*`OBJ`*) is the weighted (`w`) average of overestimation and underestimation. Here, 
* overestimation (*`OE`*) refers to the case when CPU recommendation is above the actual CPU utilization leading to slack or low utilization, while
* underestimation (*`UE`*), on the other hand, refers to the case when CPU recommendation is below CPU utilization leading to throttling events thereby reducing applications' performance.

Mathematically, 
*`OBJ`* = `w` x *`UE`* + (1-`w`) x *`OE`*, where `w` is the importance or weight assigned to underestimation relative to overestimation.

## VPA Components

Our VPA algorithm consists of the following three different components:

1. Workload characterization
2. Numerical optimization
3. Machine Learning forecast

### 1) Workload characterization

The workload characterization module analyses the past CPU utilization characteristics to estimate the weight, `w`. Mathematically, if `w`=1, then looking at the objective that would mean we value only underestimation, so recommendation *target* (`t`) will be the max of the past utilization timeseries. However, this setting would increase slack (area between recommendation and utilization) significantly, thus reducing utilization. We therefore want to set `w` close to but less than 1. The degree by which we reduce the value of `w` below 1 is determined by workload characteristics such as (i) workload periodicity and (ii) workload burstiness.

Intuitively, if a workload is periodic, it will incur higher slack for a given recommendation. Therefore, in order to reduce slack, recommendation (and therefore `w`) can be lowered down. Similarly, if a workload is bursty, we would like to further reduce `w`, otherwise it may lead to unnecessary increase in slack.

### 2) Numerical optimization

We utilize classical numerical optimization to compute the optimal recommendation (or *target* `t`) for a subset of samples in each update interval. Specifically, we minimize *`OBJ`* to obtain optimal target recommendation for past samples and formulate this minimization problem as a Mixed Integer Linear Program (*MILP*).

Mathematically, let `K` be the length of update interval and `N` be the total sample length, then the number of update intervals in the past is `M = N/K`. For each past time interval `i` in `M`, MILP computes optimal CPU recommendation targets (`t_i`). Thus, the output of this component is a vector of length `M` of optimal recommendation values for the past update intervals.

### 3) Machine learning forecast

After MILP computes the target values for the past `M` update intervals, ML modules takes these `M` optimal historical recommendations as input and predicts the optimal future recomendations for the next (one or more) update intervals depending upon the forecasting horizon `F` defined in the ML algorithm. Currently, our ML component supports simple Linear Regression, however more advanced forecast methods can be easily deployed by extending our API.
