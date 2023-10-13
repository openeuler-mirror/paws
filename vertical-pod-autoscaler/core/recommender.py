import recommender_config
from algorithm.algo_wrapper import DRIFTrecommender
from core.vpa_recommendations import VPARecommendations
from utils.utilities import str2resource, bound_var, resource2str, get_cpu_requests, get_max_trace_among_pods, \
    get_target_containers, get_target_priorities, get_ue_weight, setup_logging, \
    filter_bad_days_out, get_bad_days, get_update_interval, get_diurnal_length

LOGGER = setup_logging(__file__)

METADATA_KEY = "metadata"
SPEC = "spec"
NAME = "name"
TARGET_REF = "targetRef"
NAMESPACE = "namespace"
POD = "pod"
RESOURCE_POLICY = "resourcePolicy"
CONTAINER_POLICY = "containerPolicies"
CONTAINER = "container"
CONTAINER_NAME = "containerName"
CONTROLLED_RESOURCES = "controlledResources"
MAX_ALLOWED = "maxAllowed"
MIN_ALLOWED = "minAllowed"
SECONDS = 60
PATCHED_TIME_STAMP = "patchedTimestamp"
LOWER_BOUND = "lowerBound"
UPPER_BOUND = "upperBound"
TARGET = "target"
UNCAPPED_TARGET = "uncappedTarget"

CPU_RESOURCE_QUERY = "rate(container_cpu_usage_seconds_total{cond}[2m])"
MEMORY_RESOURCE_QUERY = "container_memory_usage_bytes{%s}"
THROTTLE_QUERY = "rate(container_cpu_cfs_throttled_seconds_total{cond_one}[2m]) / " \
                 "(rate(container_cpu_cfs_throttled_seconds_total{cond_two}[2m]) + " \
                 "ignoring(cpu) rate(container_cpu_usage_seconds_total{cond_three}[2m]))"
MEMORY = "memory"
CPU = "cpu"


class Recommender:
    drift_vpa_recommendations = VPARecommendations()
    cpu_request = 1.0

    def __init__(self, vpa, core_v1, prom_client):
        self.vpa = vpa
        self.core_v1 = core_v1
        self.prom_client = prom_client

    @property
    def get_recommendation(self):
        """
        This function takes a VPA object and query prometheus for that VPA and returns a list of recommendations
        """
        # Get VPA metadata
        vpa_metadata = self.vpa[METADATA_KEY]
        # Get the VPA spec
        vpa_spec = self.vpa[SPEC]
        vpa_name = vpa_metadata[NAME]
        update_interval_sec = get_update_interval(vpa_metadata)
        diurnal_length_sec = get_diurnal_length(vpa_metadata)

        # example target_ref {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'name': 'hamster'}
        target_ref = vpa_spec[TARGET_REF]
        deployment_name = target_ref[NAME]
        LOGGER.info(target_ref)

        # Retrieve the target pods
        if NAMESPACE in vpa_metadata.keys():
            target_namespace = vpa_metadata[NAMESPACE]
        else:
            target_namespace = recommender_config.DEFAULT_NAMESPACE

        # Build the prometheus query for the target resources of target containers in target pods
        namespace_query = NAMESPACE + "=\'" + target_namespace + "\'"

        # We need the deployment name to enable filtering a deployment within the same namespace. e.g. it is possible
        # to have two redis instances running in the  same namespace
        deployment_query = POD + "=~'" + deployment_name + ".*'"

        # Get the target containers, i.e. all the deployments using VPA
        target_containers = get_target_containers(self.core_v1, target_namespace, target_ref)
        ue_weight = get_ue_weight(vpa_metadata)
        bad_days = get_bad_days(vpa_metadata)

        # Get the target container traces
        traces = {}
        throttle_traces = {}
        recommendations = []

        for containerPolicy in vpa_spec[RESOURCE_POLICY][CONTAINER_POLICY]:
            container_queries = []
            # for all containers in this deployment
            if containerPolicy[CONTAINER_NAME] != "*":
                container_query = CONTAINER + "='" + containerPolicy[CONTAINER_NAME] + "'"
                container_queries.append(container_query)
            else:
                for container in target_containers:
                    container_query = CONTAINER + "='" + container + "'"
                    container_queries.append(container_query)

            controlled_resources = containerPolicy[CONTROLLED_RESOURCES]
            max_allowed = containerPolicy[MAX_ALLOWED]
            min_allowed = containerPolicy[MIN_ALLOWED]

            self.prom_client.update_period(recommender_config.HISTORY_LENGTH * recommender_config.FORECASTING_DAYS)
            for resource in controlled_resources:
                if resource.lower() == CPU:
                    resource_query = CPU_RESOURCE_QUERY  # rate  2 x sampling interval

                    # this query return % throttling based on throttle time i.e. throttle_time/throttle_time + runtime
                    throttle_query_seconds = THROTTLE_QUERY

                elif resource.lower() == MEMORY:
                    resource_query = MEMORY_RESOURCE_QUERY
                else:
                    LOGGER.error("Unsupported resource: " + resource)
                    break

                # Retrieve the metrics for target containers in all pods
                for container_query in container_queries:
                    # Retrieve the metrics for the target container
                    # query_index = namespace_query + "," + container_query + ", name=~\"04eb4.*\""
                    query_index = "{" + namespace_query + "," + container_query + "," + deployment_query + "}"

                    query = resource_query.format(cond=query_index)
                    # t_query = throttle_query_period.format(cond_one=query_index, cond_two=query_index)
                    t_query_seconds = throttle_query_seconds.format(cond_one=query_index, cond_two=query_index,
                                                                    cond_three=query_index)
                    LOGGER.info(query)
                    # LOGGER.info(t_query)
                    LOGGER.info(t_query_seconds)

                    # Retrieve the metrics for the target container
                    if resource.lower() == CPU:
                        traces = self.prom_client.get_prometheus_data(query, traces, resource,
                                                                      recommender_config.STEP_SIZE_DEFAULT)
                        throttle_traces = self.prom_client.get_prometheus_data(t_query_seconds, throttle_traces, resource,
                                                                               recommender_config.STEP_SIZE_THROTTLE)

            # Merge the traces for the target container belonging pods
            max_traces = get_max_trace_among_pods(traces)
            throttle_max = get_max_trace_among_pods(throttle_traces)

            # Apply the forecasting & recommendation LPrecommender, this is where our implementation should come in..
            for container in max_traces.keys():
                for resource_type in max_traces[container].keys():
                    # Sort the metrics by timestamp and extract the metrics only
                    metrics = sorted(max_traces[container][resource].items())
                    if recommender_config.DEBUG:
                        LOGGER.info(
                            "Metrics for container {} resource type {}: => {}".format(container, resource_type,
                                                                                      metrics))
                    if bad_days is not None:
                        metrics = filter_bad_days_out(metrics, bad_days)

                    # Handle throttle metrics
                    flat_throttle_metrics = sorted(throttle_max[container][resource].items())
                    cpu_request = get_cpu_requests(self.core_v1, target_namespace, target_ref,
                                                   container) or Recommender.cpu_request
                    LOGGER.info("Forecast {} resource for Container {} at {}".format(resource_type, container,
                                                                                     self.prom_client.get_current_time()))
                    current_rec_key = f'{vpa_name}-{target_namespace}-{container}-{resource_type}'
                    drift_object = Recommender.drift_vpa_recommendations.get_object(current_rec_key)
                    # convert update interval obtained from vpa object to minutes from seconds
                    if update_interval_sec:
                        update_interval_in_minutes = int(int(update_interval_sec) / SECONDS)
                        LOGGER.info("VPA update interval in mins " + str(update_interval_in_minutes))
                    else:
                        update_interval_in_minutes = int(int(recommender_config.DEFAULT_VPA_UPDATE_INTERVAL) / SECONDS)

                    if diurnal_length_sec:
                        diurnal_length_in_minutes = int(int(diurnal_length_sec) / SECONDS)
                    else:
                        diurnal_length_in_minutes = int(int(recommender_config.DEFAULT_DIURNAL_LENGTH) / SECONDS)

                    if drift_object is None:
                        drift_object = DRIFTrecommender(update_interval_in_minutes, diurnal_length_in_minutes,
                                                        ue_weight, cpu_request)
                        Recommender.drift_vpa_recommendations.set_object(current_rec_key, drift_object)

                    patched_timestamp, lower_bound, uncapped_target, upper_bound = drift_object.drift_recommender(
                        metrics,
                        flat_throttle_metrics)
                    LOGGER.info("current recommender key : " + current_rec_key)
                    if patched_timestamp == -1:
                        LOGGER.warning("No recommendation returned")
                        continue

                    LOGGER.info(
                        "LP Recommendation: Container: " + container + " Lower Bound: " + str(
                            lower_bound) + " Uncapped Target: " + str(uncapped_target) +
                        " Upper Bound: " + str(upper_bound))

                    container_recommendation = {PATCHED_TIME_STAMP: patched_timestamp, CONTAINER_NAME: container,
                                                LOWER_BOUND: {}, TARGET: {},
                                                UNCAPPED_TARGET: {}, UPPER_BOUND: {}}
                    # If the target is below the lower_bound, set it to the lower_bound
                    min_allowed_value = str2resource(resource, min_allowed[resource])
                    max_allowed_value = str2resource(resource, max_allowed[resource])
                    target = bound_var(uncapped_target, min_allowed_value, max_allowed_value)
                    lower_bound = bound_var(lower_bound, min_allowed_value, max_allowed_value)
                    upper_bound = bound_var(upper_bound, min_allowed_value, max_allowed_value)

                    container_recommendation[LOWER_BOUND][resource] = resource2str(resource, lower_bound)
                    container_recommendation[TARGET][resource] = resource2str(resource, target)
                    container_recommendation[UNCAPPED_TARGET][resource] = resource2str(resource, uncapped_target)
                    container_recommendation[UPPER_BOUND][resource] = resource2str(resource, upper_bound)
                    recommendations.append(container_recommendation)
        return recommendations
