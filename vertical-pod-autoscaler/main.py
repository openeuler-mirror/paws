import datetime
import os

from kubernetes import client, config
from kubernetes.client.rest import ApiException

import recommender_config
from core.core_exception import NoVPAObjectException
from core.prom_crawler import PromCrawler
from core.recommender import Recommender
from utils.utilities import setup_logging, sleep_until, select_recommender, is_time_to_get_recommendations, \
    get_update_interval, get_vpa_creation_time, get_vpa_last_updated, time_before_next_update, \
    get_vpa_last_updated_or_created

DEBUG = recommender_config.DEBUG
DOMAIN = "autoscaling.k8s.io"
VPA_NAME = "verticalpodautoscaler"
VPA_PLURAL = "verticalpodautoscalers"
LOGGER = setup_logging(__file__)


if __name__ == '__main__':
    if 'KUBERNETES_PORT' in os.environ:
        config.load_incluster_config()
    else:
        config.load_kube_config()

    # Get the api instance to interact with the cluster
    api_client = client.api_client.ApiClient()
    v1 = client.ApiextensionsV1Api(api_client)
    corev1 = client.CoreV1Api(api_client)
    crds = client.CustomObjectsApi(api_client)
    resource_version = ''

    # Initialize the prometheus client, if PROM_TOKEN is set then use the right constructor
    if recommender_config.PROM_TOKEN != "":
        prom_client = PromCrawler(recommender_config.PROM_URL, recommender_config.PROM_TOKEN)
    else:
        # use the core URL configured in config, this wouldn't work when this driver is running outside the cluster.
        # --sc e.g. if you are testing by running this main file replace the one in config with e.g.
        # http://1.1.1.1:9090, of course after port forwarding.
        prom_client = PromCrawler(recommender_config.PROM_URL)
        # user forwarded address, mainly for local testing --sc
        # prom_client = PromCrawler()

    # Get the VPA CRD, this will check if VPA functionality is installed in k8s. --sc
    # The equivalent kubectl command is kubectl get crds -o jsonpath='{.items[*].spec.names.kind}' --sc
    # https://kubernetes.io/docs/reference/kubectl/jsonpath/
    # If there is no VPA functionality installed then exit. --sc
    current_crds = [x['spec']['names']['kind'].lower() for x in v1.list_custom_resource_definition().to_dict()['items']]
    if VPA_NAME not in current_crds:
        LOGGER.error("VPA is not currently installed in the cluster. To proceed, install VPA first")
        raise NoVPAObjectException()
        exit(-1)
    current_time = datetime.datetime.now().today()

    while True:
        # This is going to make sure we only provide recommendation when the need arises.
        sleep_until(current_time)
        # kubectl get verticalpodautoscalers -o json
        vertical_pod_autoscalers = crds.list_cluster_custom_object(group=DOMAIN, version="v1", plural=VPA_PLURAL)
        if DEBUG:
            LOGGER.info(vertical_pod_autoscalers)

        # Get ONLY the custom VPAs by iteration through the VPA object.
        # This function will not return object that has no recommenders field so it filters out only VPAs with
        # matching recommender name.
        # There could be several VPAs Running in the cluster but a Deployment can be attached to only one VPA.
        selected_vpas = select_recommender(vertical_pod_autoscalers, recommender_config.RECOMMENDER_NAME)
        if len(selected_vpas) > 0:
            LOGGER.info(" ================= Selected VPAs =============== ")
            LOGGER.info(selected_vpas)
            # For each vpa object, check if recommendations are available, if yes provide recommendations
            for vpa in selected_vpas:
                # Get the name of the vpa and namespace
                vpa_metadata = vpa["metadata"]
                vpa_name = vpa_metadata["name"]
                vpa_namespace = vpa_metadata["namespace"]
                update_interval_sec = get_update_interval(vpa_metadata)
                vpa_creation_time = get_vpa_creation_time(vpa_metadata)
                vpa_last_updated_time = get_vpa_last_updated(vpa_metadata)
                vpa_creation_update = get_vpa_last_updated_or_created(vpa_metadata)

                # get recommendations
                # The Recommender take drift obeject and drift_vpa_recommendation to
                paws_recommender = Recommender(vpa, corev1, prom_client)

                # If update interval is not specified in the vpa object, just fetch recommendations
                if not update_interval_sec:
                    recommendations = paws_recommender.get_recommendation
                elif update_interval_sec and is_time_to_get_recommendations(vpa, vpa_creation_update):
                    recommendations = paws_recommender.get_recommendation
                elif not is_time_to_get_recommendations(vpa, vpa_creation_update):
                    target_time = vpa_last_updated_time if vpa_last_updated_time else vpa_creation_time
                    LOGGER.info(
                        f'Not time to give recommendations: {time_before_next_update(vpa_metadata, target_time)} '
                        f'seconds more before providing a new recommendation for VPA object {vpa_name}')
                    continue

                if not recommendations:
                    LOGGER.info("No new recommendations obtained, so skip updating the VPA object {}".format(vpa_name))
                    continue

                patched_timestamp =  recommendations[0]["patchedTimestamp"]
                LOGGER.info(" " + str(patched_timestamp))
                patched_ts = datetime.datetime.utcfromtimestamp(patched_timestamp).strftime('%Y-%m-%d %H:%M:%S')
                patched_annotation = {"lastUpdate": patched_ts}
                # Update the recommendations.
                patched_vpa = {"recommendation": {"containerRecommendations": recommendations}}
                body = {"status": patched_vpa, "metadata": {"annotations": patched_annotation}}

                # Update the VPA object API call doc: https://github.com/kubernetes-client/python/blob/master/kubernetes
                # /docs/CustomObjectsApi.md#patch_namespaced_custom_object
                try:
                    vpa_updated = crds.patch_namespaced_custom_object(group=DOMAIN, version="v1", plural=VPA_PLURAL,
                                                                         namespace=vpa_namespace, name=vpa_name,
                                                                         body=body)
                    LOGGER.info("Successfully patched VPA object ( " + vpa_name + " ) with the recommendation: %s" %
                                vpa_updated['status']['recommendation']['containerRecommendations'])
                except ApiException as e:
                    LOGGER.error("Exception when calling CustomObjectsApi->patch_namespaced_custom_object: %s\n" % e)
            # current_time = current_time + datetime.timedelta(minutes=5)
            current_time = current_time + datetime.timedelta(seconds=recommender_config.SLEEP_WINDOW)
            LOGGER.info("=================== Next execution will be at ===================" + str(current_time))
        else:
            LOGGER.info(
                "There are no VPA objectS for the recommender '" + recommender_config.RECOMMENDER_NAME + "' to process")
            current_time = current_time + datetime.timedelta(seconds=recommender_config.SLEEP_WINDOW)
            LOGGER.info("=================== Next execution will be at ===================" + str(current_time))
