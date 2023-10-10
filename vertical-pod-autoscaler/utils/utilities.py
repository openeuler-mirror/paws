import math
import os
import errno
import logging
import time
from pprint import pprint

import numpy as np
from datetime import datetime, timedelta, timezone
import pause
from matplotlib import pyplot as plt
import pandas as pd

# Local imports
import recommender_config

RECOMMENDERS = "recommenders"
NAME = "name"
LABELS = "labels"

# Utility functions for Core modules
# Setup global logging
def setup_logging(file):
    logger = logging.getLogger(file)
    if not logger.handlers:
        logger.setLevel(logging.DEBUG)
        # create console handler and set level to debug
        ch = logging.StreamHandler()
        ch.setLevel(logging.DEBUG)

        # create formatter
        formatter = logging.Formatter('%(asctime)s - %(name)s:%(lineno)d - %(levelname)s - %(message)s')

        # add formatter to ch
        ch.setFormatter(formatter)

        # add ch to logger
        logger.addHandler(ch)
    return logger


LOGGER = setup_logging(__file__)


def select_recommender(vpas, recommender_name):
    selected_vpas = []
    for vpa in vpas["items"]:
        vpa_spec = vpa["spec"]
        if RECOMMENDERS not in vpa_spec.keys():
            continue
        else:
            LOGGER.info(vpa_spec)
            for recommender in vpa_spec[RECOMMENDERS]:
                if recommender[NAME] == recommender_name:
                    selected_vpas.append(vpa)

    return selected_vpas


# resource_2str converts a resource (CPU, Memory) value to a string
def resource2str(resource, value):
    if resource.lower() == "cpu":
        if value < 1:
            return str(int(math.ceil(value * 1000))) + "m"
        else:
            return str(value)
    # Memory is in bytes
    else:
        if value < 1024:
            return str(value) + "B"
        elif value < 1024 * 1024:
            return str(int(value / 1024)) + "k"
        elif value < 1024 * 1024 * 1024:
            return str(int(value / 1024 / 1024)) + "Mi"
        else:
            return str(int(value / 1024 / 1024 / 1024)) + "Gi"


# Convert a resource (CPU, Memory) string to a float value
def str2resource(resource, value):
    if type(value) is str:
        if resource.lower() == "cpu":
            if value[-1] == "m":
                return float(value[:-1]) / 1000
            else:
                return float(value)
        else:
            if value[-1].lower() == "b":
                return float(value[:-1])
            elif value[-1].lower() == "k":
                return float(value[:-1]) * 1024
            elif value[-2:].lower() == "mi":
                return float(value[:-2]) * 1024 * 1024
            elif value[-2:].lower() == "gi":
                return float(value[:-2]) * 1024 * 1024 * 1024
            else:
                return float(value)
    else:
        return value


def str_to_val(cpu):
    if cpu[-1] == "m":
        return float(cpu[:-1]) / 1000
    else:
        return float(cpu)


def get_target_priorities(corev1_client, target_namespace, target_ref):
    pods = corev1_client.list_namespaced_pod(namespace=target_namespace,
                                             label_selector="app=" + target_ref[NAME])
    priority_class = []
    for pod in pods.items:
        if pod.spec.priority_class_name:
            priority_class.append(pod.spec.priority_class_name)
    return priority_class


def get_ue_weight(vpa_metadata):
    if LABELS in vpa_metadata:
        return vpa_metadata["labels"].get("weight", None)
    else:
        return None


def get_cpu_requests(corev1_client, target_namespace, target_ref, container):
    pods = corev1_client.list_namespaced_pod(namespace=target_namespace,
                                             label_selector="app=" + target_ref[NAME])
    for pod in pods.items:
        for c in pod.spec.containers:
            if c.name == container:
                requests = c.resources.requests
                return str_to_val(requests['cpu'])
    return 1.0


def get_target_containers(corev1_client, target_namespace, target_ref):
    # Multiple deployments (pods) can subscribe or share the same VPA. So for each deployment, we extract targeted
    # containers this is the case if we don't containerName set to *.
    target_pods = corev1_client.list_namespaced_pod(namespace=target_namespace,
                                                    label_selector="app=" + target_ref[NAME])

    # Retrieve the target containers
    target_containers = []
    for pod in target_pods.items:
        for container in pod.spec.containers:
            if container.name not in target_containers:
                target_containers.append(container.name)

    return target_containers


def get_max_trace_among_pods(traces):
    max_traces = {}
    for container in traces.keys():
        max_traces[container] = {}
        for resource_type in traces[container].keys():
            max_traces[container][resource_type] = {}
            for pod in traces[container][resource_type].keys():
                cur_trace = traces[container][resource_type][pod]
                for data in cur_trace:
                    if data[0] not in max_traces[container][resource_type].keys():
                        max_traces[container][resource_type][data[0]] = float(data[1])
                    else:
                        max_traces[container][resource_type][data[0]] = max(float(data[1]),
                                                                            max_traces[container][resource_type][
                                                                                data[0]])

    return max_traces


def bound_var(var, min_value, max_value):
    if var < min_value:
        return min_value
    elif var > max_value:
        return max_value
    else:
        return var


def flatten_list(metrics):
    result = [i[1:] for i in metrics]
    flat_list = [item for sublist in result for item in sublist]
    return np.asarray(flat_list)


def plot_util(metrics, filename):
    plt.plot(metrics)
    plt.savefig(filename)
    plt.close()


def list_to_csv(lst):
    with open('data.csv', 'w') as f:
        f.write(str(lst))


# Utility functions for the PromCrawler class
def construct_nested_dict(traces_dict, container, resourcetype, pod=None):
    if pod is None:
        if container not in traces_dict.keys():
            traces_dict[container] = {resourcetype: []}
        elif resourcetype not in traces_dict[container].keys():
            traces_dict[container][resourcetype] = []
    else:
        if container not in traces_dict.keys():
            traces_dict[container] = {resourcetype: {pod: []}}
        elif resourcetype not in traces_dict[container].keys():
            traces_dict[container][resourcetype] = {pod: []}
        elif pod not in traces_dict[container][resourcetype].keys():
            traces_dict[container][resourcetype][pod] = []

    return traces_dict


def get_key_names(attribute, klist):
    keys = [kname for kname in klist if attribute in kname.lower()]
    if len(keys) > 0:
        return keys
    else:
        return []


def sleep_until(current_time):
    pause.until(datetime(current_time.year, current_time.month, current_time.day, current_time.hour,
                         current_time.minute))
    LOGGER.info("**** VPA Executed ****")


# This function returns an array of bad data days to exclude from prometheus data
def get_bad_days(vpa_metadata):
    if "labels" in vpa_metadata and "bad_days" in vpa_metadata["labels"]:
        bad_days = list(map(int, vpa_metadata["labels"].get("bad_days", None).split("-")))
        return bad_days
    else:
        return None


def day_to_date(bad_day):
    fmt = "%Y-%m-%d %H:%M:%S"
    d = datetime.today() - timedelta(days=bad_day)
    return datetime.strptime(d.strftime(fmt), fmt)


# If bad days is specified in vpa object, our recommender should filter these days and based on timestamp..
# E.g. if 1,2,3 are considered bad days out of the seven days
# The input we get from prometheus is : [[ts: value], [ts: value], [ts: value]]
def filter_bad_days_out(metrics, bad_days):
    for bd in bad_days:
        bad_day = day_to_date(bd)
        bad_day_unix_ts = bad_day.replace(tzinfo=timezone.utc).timestamp()
        for current_ts in metrics[:]:
            bad_day = datetime.fromtimestamp(int(bad_day_unix_ts))
            ts_day = datetime.fromtimestamp(int(current_ts[0]))
            same_day = bad_day.date() == ts_day.date()
            if same_day:
                metrics.remove(current_ts)
    return metrics


# How  do we know we have to update every x hours?
# We have to store this information per vpa and hence the need for annotation or labels.
# Value returned from this function is in seconds
def get_update_interval(vpa_metadata):
    if "labels" in vpa_metadata and "update_interval_sec" in vpa_metadata["labels"]:
        return vpa_metadata['labels']['update_interval_sec']
    elif hasattr(recommender_config, "DEFAULT_VPA_UPDATE_INTERVAL"):
        return recommender_config.DEFAULT_VPA_UPDATE_INTERVAL
    return None


# Half life or periodicity definition
def get_diurnal_length(vpa_metadata):
    if "labels" in vpa_metadata and "diurnal_length_sec" in vpa_metadata["labels"]:
        return vpa_metadata['labels']['diurnal_length_sec']
    elif hasattr(recommender_config, "DEFAULT_DIURNAL_LENGTH"):
        return recommender_config.DEFAULT_DIURNAL_LENGTH
    return None


# Here the logic is, if the difference between current time and now is greater than the update interval (time to
# wait before the vpa object is patched, then apply the patch).. Example
# vpa_date = 29-08-2023:17:00:00
# now = 29-08-2023:20:00:00
# update interval is 3 hours == 10800 seconds, in this case 29-08-2023:20:00:00 - 29-08-2023:17:00:00 = 3 hrs
def get_time_difference_between_last_patch_and_now(vpa_date):
    vpa_date = vpa_date.replace("Z", "")
    date_format = '%Y-%m-%dT%H:%M:%S'
    try:
        date_obj = datetime.strptime(vpa_date, date_format)
    except Exception as e:
        LOGGER.warning(e)
        date_format = '%Y-%m-%d %H:%M:%S'
        date_obj = datetime.strptime(vpa_date, date_format)
    time_since_last_update = (datetime.utcnow() - date_obj).total_seconds()
    return time_since_last_update


def time_before_next_update(vpa_metadata, update_time):
    update_interval_sec = get_update_interval(vpa_metadata) or recommender_config.DEFAULT_VPA_UPDATE_INTERVAL
    time_since_last_update = get_time_difference_between_last_patch_and_now(update_time)
    time_left = float(update_interval_sec) - time_since_last_update
    return int(time_left)


# checks if it is time to patch vpa object
def is_time_to_get_recommendations(vpa, update_time):
    vpa_metadata = vpa['metadata']
    update_interval_sec = get_update_interval(vpa_metadata) or recommender_config.DEFAULT_VPA_UPDATE_INTERVAL
    time_since_last_update = get_time_difference_between_last_patch_and_now(update_time)
    LOGGER.info("Time since last update : " + str(time_since_last_update))
    second_epsilon = 1
    if time_since_last_update + second_epsilon >= int(update_interval_sec):
        return True
    return False


# Get vpa creation time
def get_vpa_creation_time(vpa_metadata):
    return vpa_metadata['creationTimestamp']


# Get the last time vpa object was patched
def get_vpa_last_updated(vpa_metadata):
    if "annotations" in vpa_metadata and "lastUpdate" in vpa_metadata["annotations"]:
        return vpa_metadata["annotations"]["lastUpdate"]
    return None


# Return the last time the vpa object was patched with new recommendations
# If lastUpdated is not present then the vpa creation date will be used.
def get_vpa_last_updated_or_created(vpa_metadata):
    vpa_creation_time = get_vpa_creation_time(vpa_metadata)
    vpa_last_updated_time = get_vpa_last_updated(vpa_metadata)
    vpa_creation_update = vpa_last_updated_time or vpa_creation_time
    return vpa_creation_update


def round_time(dt=None, round_to=1):
    """Round a datetime object to any time-lapse in seconds
    dt : datetime.datetime object, default now.
    roundTo : Closest number of seconds to round to, default 15 seconds.
    """
    if dt is None:
        dt = datetime.now()

    seconds = dt.replace(second=0, microsecond=0).timestamp()
    remainder = seconds % (round_to * 15)
    return seconds - remainder


def get_now_str():
    now = datetime.now()
    now_str = now.strftime("%Y%m%d-%H%M")
    return now_str


# Some basic testing
if __name__ == '__main__':
    # df = pd.Series(np.random.randn(10000), index=pd.date_range("2023-07-15", periods=10000, freq="min"),
    # name="cpu_metrics").to_frame() df_filtered = df.loc['2023-07-15'] print(df_filtered)
    metrics = [(1689848324, 0.05872891927533386), (1689848324, 0.34289399633809026), (1689848324, 0.5568551209502992),
               (1689757140, 0.5399747714724069), (1689757140, 0.5490833788776499), (1689757140, 0.5615630285623934),
               (1689675524, 0.5563249595969372), (1689675524, 0.6287228545747306), (1689675524, 0.7071035346003894),
               (1689589124, 0.37206032533141825), (1689589124, 0.3764170563228719), (1689589124, 0.36668086856866333),
               (1689502724, 0.3825796927213047), (1689502724, 0.3996932014011861), (1689502724, 0.41215708373835147),
               (1689416324, 0.4309865338099163), (1689416324, 0.5016293200276778), (1689416324, 0.5465527997642656),
               (1689324095, 0.713585768767996), (1689324095, 0.7162943677193901), (1689324095, 0.5264954292503157)]
    pprint(filter_bad_days_out(metrics, [1, 2, 4, 7]))
