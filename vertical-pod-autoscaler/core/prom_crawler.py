import os
from datetime import datetime

import requests

from utils.utilities import get_key_names, construct_nested_dict, setup_logging, round_time

LOGGER = setup_logging(__file__)

# The PromCrawler enable connecting to a local or in kubernetes prometheus instance.
# Once connected, queries are executed to extract metrics information
STATUS = 'status'
ERROR = 'error'
PROM_HOST = "PROM_HOST"
PROM_TOKEN = "PROM_TOKEN"
PROM_ADDRESS = "http://127.0.0.1:9090"
CRAWLING_PERIOD_SEC = 86400
QUERY = 'query'
START = 'start'
END = 'end'
STEP = 'step'
DAY_SECONDS = 86400


class PromCrawler:
    now = None
    start = None
    # Default values, in production set these values in the configuration file (config/recommender_config.yaml)
    prom_address = PROM_ADDRESS
    crawling_period = CRAWLING_PERIOD_SEC  # Default crawling of one day i.e. Get the last 24 hours
    prom_token = None
    end = None

    def __init__(self, prom_address=None, prom_token=None):
        # used the passed core address or prom_host env or default back to the instance declared.
        self.prom_address = prom_address or os.getenv(PROM_HOST) or self.prom_address
        self.prom_token = prom_token or os.getenv(PROM_TOKEN)

        if not self.prom_address or not self.crawling_period:
            LOGGER.info("Please configure $PROM_HOST, $PROM_TOKEN, $CRAWLING_PERIOD to successfully run the crawler!")
            raise ValueError(f'Please make sure to set PROM_HOST {self.prom_address} and '
                             f'CRAWLING_PERIOD {self.crawling_period} properly')

    def update_period(self, crawling_period):
        LOGGER.info(
            "CRAWLING PERIOD " + str(crawling_period) + " period is " + str(crawling_period / DAY_SECONDS) + " days")
        self.crawling_period = crawling_period
        self.now = int(round_time(dt=datetime.now()))
        self.start = int(self.now - self.crawling_period)  # last day
        self.end = self.now

    def get_current_time(self):
        current_time_str = datetime.fromtimestamp(self.now).strftime("%I:%M:%S")
        return current_time_str

    # Prometheus can only return 11K points for a single query, e.g. when step is set to 60s for 7 days the number of
    # points returned will be more than this threshold resulting to an error. So we have increased the step size to
    # at least 120s. step = '120s'
    def fetch_data_range(self, my_query, start, end, step):
        try:
            if self.prom_token:
                headers = {"content-type": "application/json; charset=UTF-8",
                           'Authorization': 'Bearer {}'.format(self.prom_token)}
            else:
                headers = {"content-type": "application/json; charset=UTF-8"}
            full_query = '{0}/api/v1/query_range'.format(self.prom_address)

            response = requests.get(full_query,
                                    params={QUERY: my_query, START: start + 1, END: end, STEP: step},
                                    headers=headers, verify=False)
            LOGGER.debug(response)

        except requests.exceptions.RequestException as e:
            LOGGER.error(e)
            return None

        try:
            if response.json()[STATUS] != "success":
                LOGGER.error("Error processing the request: " + response.json()[STATUS])
                LOGGER.error("The Error is: " + response.json()[ERROR])
                return None

            results = response.json()['data']['result']

            if results is None or len(results) <= 0:
                return None
            return results
        except Exception:
            LOGGER.error(response)
            return None

    def fetch_data_range_in_chunks(self, my_query, step):
        all_metric_history = []
        trials = 0
        cur_metric_history = None
        while (cur_metric_history is None) and (trials < 3):
            cur_metric_history = self.fetch_data_range(my_query, self.start, self.end, step)
            trials += 1
            if cur_metric_history is not None:
                all_metric_history += cur_metric_history

        return all_metric_history

    def get_until(self, map_object, candidates):
        for cand in candidates:
            obj = map_object.get(cand, None)
            if obj is not None:
                return obj
        return None

    def get_prometheus_data(self, query, traces, resource_type, step_size):
        cur_trace = self.fetch_data_range_in_chunks(query, step_size)

        # Convert the prometheus data to a list of floats
        try:
            metric_obj_attributes = cur_trace[0]["metric"].keys()
        except:
            LOGGER.error("There are no data points for metric query {}.".format(query))
            return traces

        pod_key_names = get_key_names("pod", metric_obj_attributes)
        container_key_names = get_key_names("container", metric_obj_attributes)
        ns_key_names = get_key_names("namespace", metric_obj_attributes)

        if len(pod_key_names) == 0 or len(container_key_names) == 0 or len(ns_key_names) == 0:
            LOGGER.warning(
                "[Warning] The metric object returned from Prometheus query {} does not have required attribute tags.".format(
                    query))
            LOGGER.warning("[Warning] The following attributes to the metric should not be empty.")
            LOGGER.warning("[Warning] - pod attribute name: {}".format(pod_key_names))
            LOGGER.warning("[Warning] - container attribute name: {}".format(container_key_names))
            LOGGER.warning("[Warning] - namespace attribute name: {}".format(ns_key_names))

        for metric_obj in cur_trace:
            pod = self.get_until(metric_obj["metric"], pod_key_names)
            if pod is None:
                continue
            container = self.get_until(metric_obj["metric"], container_key_names)
            if container is None:
                continue

            metrics = metric_obj['values']
            traces = construct_nested_dict(traces, container, resource_type, pod)
            traces[container][resource_type][pod] += metrics
        return traces
