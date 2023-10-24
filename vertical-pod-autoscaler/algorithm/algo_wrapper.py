# Copyright (c) Huawei Technologies Co., Ltd. 2023. All rights reserved.
# paws licensed under the Mulan PSL v2.
# You can use this software according to the terms and conditions of the Mulan PSL v2.
# You may obtain a copy of Mulan PSL v2 at:
#     http://license.coscl.org.cn/MulanPSL2
# THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
# PURPOSE.
# See the Mulan PSL v2 for more details.
# Author: Rajkarn Singh
# Create: 2023-09-25
# Description: Wrapper API for VPA recommender

import numpy as np
import pandas as pd

from algorithm.data_characterization import DataCharacterization
from algorithm.milp_recommender import MILPRecommender
from algorithm.ml_component import MLPredictor
from utils.utilities import setup_logging

LOGGER = setup_logging(__file__)

class DRIFTrecommender():
    """ Entry point of the algorithm API

    Attributes
    ----
        optimization_interval : int
            Length of auto-mode update interval (in minutes)
        diurnal_len_in_min: int
            Periodicity (assuming diurnal on daily basis = 1440)
        ue_weight : float
            Importance or weight of Under Estimation
        cpu_request : float
            Absolute value of requested CPU resource
        n_forecast_intervals : int
            Forecasting horizon. In minutes, it is equal to (optimization_interval * n_forecast_intervals)
    """

    def __init__(self, optimization_interval, diurnal_len_in_min, ue_weight=None, cpu_request=1.0, n_forecast_intervals = 1):
        self.SECONDS_PER_MINUTE = 60
        self.LOWER_BOUND_FACTOR = 0.1 # equivalent to 10%
        self.UPPER_BOUND_FACTOR = 0.05 # equivalent to 5% 

        self.ue_weight = ue_weight
        self.cpu_request = cpu_request
        self.optimization_interval = optimization_interval
        self.diurnal_len_in_min = diurnal_len_in_min
        self.n_forecast_intervals = n_forecast_intervals

        self.base_ts = -1 # Initial  timestamp record for this object
        self.lower_bound = -1
        self.target = -1
        self.upper_bound = -1
        self.dict_past_targets = {self.optimization_interval: pd.DataFrame(columns=['time', 'target'])}

    def drift_recommender(self, usage, throttles):
        """Compute the CPU utilization recommendation for rightsizing
        Args
        ----
            usage : list of (ts, cpu utilization) tuples
                Expects timestamp to be UNIX time in seconds
            throttles: list of (ts, throttle) tuples
                Expects throttles to be percentage time a pod is throttled
        Returns
        -------
            lowerBound : float 
                lower value of recommendation
            target : float
                CPU recommendation
            upperBound : float
                upper value of recommendation
        """

        # Handle missing values
        LOGGER.debug(f"Length of input UTIL samples: {len(usage)}")

        # Process trace and throttle data: (1) add missing timestamps first, (2) interpolate missing values.
        df_usage_all_ts, df_throttles_all_ts = self.add_missing_timestamps(usage, throttles)
        df_usage_all_ts, df_throttles_all_ts = self.interpolate_missing_values(df_usage_all_ts, df_throttles_all_ts)

        # Get the dataframe from the saved dictionary of (ts, targets)
        df_past_targets = self.dict_past_targets[self.optimization_interval]

        # First sample-length processing:
        # Check if this object has the targets computed for some of the past timestamps.
        # If yes, accordingly shorten the dataframe for MILP.
        if len(df_past_targets['time']) == 0:
            LOGGER.debug("No previous data for the object")
            df_usage_selective_ts = df_usage_all_ts.copy()
            df_throttles_selective_ts = df_throttles_all_ts.copy()
            next_expected_ts = -1
        else:
            LOGGER.debug("History for VPA object exists")
            last_ts = max(df_past_targets['time'])
            next_expected_ts = last_ts + (self.optimization_interval * self.SECONDS_PER_MINUTE)
            LOGGER.debug(f"Last OPT interval timestamp: {last_ts}")
            LOGGER.debug(f"Next expected OPT interval timestamp: {next_expected_ts}")

            df_usage_selective_ts = df_usage_all_ts.loc[df_usage_all_ts['time'] >= next_expected_ts]
            df_throttles_selective_ts = df_throttles_all_ts.loc[df_throttles_all_ts['time'] >= next_expected_ts]

        LOGGER.debug(f"New data samples length after first processing: {len(df_usage_selective_ts)}")

        # Second sample-length processing:
        # Removing additional samples if not a multiple of self.optimization_interval
        if (len(df_usage_selective_ts) % self.optimization_interval) != 0:
            LOGGER.warning(f"Since number of samples is not a multiple of {self.optimization_interval}, discarding additional datasamples at the end!")
            remainder = len(df_usage_selective_ts) % self.optimization_interval
            df_usage_selective_ts_final = df_usage_selective_ts.iloc[0:-remainder]
            df_throttles_selective_ts_final = df_throttles_selective_ts.iloc[0:-remainder]
            LOGGER.debug(f"New data samples length after second processing: {len(df_usage_selective_ts_final)}")
        else:
            df_usage_selective_ts_final = df_usage_selective_ts.copy()
            df_throttles_selective_ts_final = df_throttles_selective_ts.copy()

        if len(df_usage_selective_ts_final) < self.optimization_interval: # Should be 0 after second processing
            LOGGER.warning(f"Samples are less than {self.optimization_interval} - returning the previous recommendation")
            lower_bound = self.lower_bound
            target = self.target
            upper_bound = self.upper_bound
            return [next_expected_ts, lower_bound, target, upper_bound]

        if next_expected_ts == -1:
            remainder = len(df_usage_selective_ts) % self.optimization_interval
            expected_patched_ts = max(df_usage_selective_ts_final['time']) + self.SECONDS_PER_MINUTE
        else:
            expected_patched_ts = next_expected_ts + (len(df_usage_selective_ts_final) * self.SECONDS_PER_MINUTE)

        LOGGER.debug(f"Next expected VPA PATCH interval timestamp: {expected_patched_ts}")

        # Proceed ahead since all preprocessed data satisfies the conditions for obtaining a new recommendation
        ts_processed = df_usage_selective_ts_final['time'].values
        usage_processed = df_usage_selective_ts_final['cpu_usage'].values
        throttles_processed = df_throttles_selective_ts_final['throttles'].values

        ts_opt_intervals = ts_processed[::self.optimization_interval]
        self.base_ts = ts_opt_intervals[0]
        LOGGER.debug(f"Number of new OPTIMIZATION intervals: {len(ts_opt_intervals)}")
        LOGGER.debug(f"First timestamp of newly received OPT interval: {self.base_ts}")
        LOGGER.debug(f"Last timestamp of newly received OPT interval: {ts_opt_intervals[-1]}")
        LOGGER.debug(f"Last timestamp of UTIL samples: {ts_processed[-1]}")

        ## Call core algorithm API components
        # Component-1: Data Characterization
        if self.ue_weight == None: # This gets called only the first time when the object is new
                characterization = DataCharacterization(usage_processed, self.optimization_interval, self.diurnal_len_in_min,
                                                        priority=None, cpu_request=self.cpu_request)
                self.ue_weight, isperiodic, spread = characterization.get_workload_ue_weight()
                LOGGER.debug(f"UE Weight: {self.ue_weight}, Spread: {spread}, Is periodic: {isperiodic}")

        # Component-2: Numerical Optimization
        milp_recommender = MILPRecommender(usage_processed, throttles_processed, self.optimization_interval,
                                           ue_weight=self.ue_weight, cpu_request=self.cpu_request)
        target_new_intervals = milp_recommender.milp_recommender()
        LOGGER.debug(f"Length of MILP returned samples: {len(target_new_intervals)}")

        assert len(ts_opt_intervals) == len(target_new_intervals), f"Number of optimization intervals and number of targets mismatch"

        df_new_targets = pd.DataFrame(list(zip(ts_opt_intervals, target_new_intervals)), columns=['time', 'target'])

        # Append the newly computed MILP recommendations to the past recommendations
        df_past_targets = pd.concat([df_past_targets, df_new_targets], axis=0)
        LOGGER.debug(f"Updated (TS, REC): {df_past_targets}")
        self.dict_past_targets[self.optimization_interval] = df_past_targets

        # Component-3: ML based prediction component
        list_all_past_targets = df_past_targets['target'].tolist()
        ml_predictor = MLPredictor(list_all_past_targets, forecast_horizon=self.n_forecast_intervals)
        target = ml_predictor.predictor_runner()

        # Compute upper and lower bounds
        lower_bound = target - (self.LOWER_BOUND_FACTOR * target)
        upper_bound = target + (self.UPPER_BOUND_FACTOR * target)

        self.lower_bound = lower_bound
        self.target = target
        self.upper_bound = upper_bound

        LOGGER.debug(f"AlgoWrapper returns: {expected_patched_ts}, {lower_bound}, {target}, {upper_bound}")
        return [expected_patched_ts, lower_bound, target, upper_bound]

    def add_missing_timestamps(self, usage, throttles):
        # Make df out of tuples
        df_usage = pd.DataFrame(usage, columns = ['time', 'cpu_usage'])
        df_throttles = pd.DataFrame(throttles, columns = ['time', 'throttles'])

        min_ts = min(df_usage['time'][0], df_throttles['time'][0])
        max_ts = max(df_usage['time'][len(df_usage)-1], df_throttles['time'][len(df_throttles)-1])

        # Calculate time keys assuming that input time is in seconds
        time_keys = list(np.arange(min_ts, max_ts + self.SECONDS_PER_MINUTE - 1, self.SECONDS_PER_MINUTE))

        # Create a dummy dataframe with all time keys from min_ts to max_ts
        df_dummy = pd.DataFrame(time_keys, columns=['time'])
        
        # Merge input dataframe with dummy df for all timestamps
        df_usage_all_ts = pd.merge(df_dummy, df_usage, how='left', on=['time'])
        df_throttles_all_ts = pd.merge(df_dummy, df_throttles, how='left', on=['time'])

        return [df_usage_all_ts, df_throttles_all_ts]
    
    def interpolate_missing_values(self, df_usage_all_ts, df_throttles_all_ts):
        # Interpolate values for the newly added missing timestamps
        df_usage_all_ts = df_usage_all_ts.interpolate()
        df_throttles_all_ts = df_throttles_all_ts.interpolate()

        return [df_usage_all_ts, df_throttles_all_ts]