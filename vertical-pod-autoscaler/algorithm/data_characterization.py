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
# Description: Data Characterization API required for computing weights input to DRIFT recommender

import numpy as np

# Local imports
from algorithm.utilities import curate_samples
from utils.utilities import setup_logging

LOGGER = setup_logging(__file__)

class DataCharacterization():
    PRIORITY_IMPACT_HIGH = 0.0
    PRIORITY_IMPACT_MID = 0.2
    PRIORITY_IMPACT_LOW = 0.4

    MAX_STD = 0.5 # Maximum standard deviation for a [0,1] sequence is always 0.5
    POWER_SPECTRUM_EXPONENT = 2
    FFT_SAMPLING_SPACE = 1
    POWER_COMPONENTS_ACCOUNTED = -5
    PERIOD_POWER_THR = 0.1

    HIGH_PRIORITY = 'high'
    LOW_PRIORITY = 'low'
    MID_PRIORITY = 'mid'

    REC_LOWER_VALUE = 0.90
    REC_UPPER_VALUE = 0.99
    PERIODICITY_IMPACT = 0.02
    SPREAD_IMPACT = 0.05
    
    def __init__(self, samples, optimization_interval, diurnal_len_in_min, priority=None, cpu_request=1.0):
        self.samples = samples
        self.priority = priority
        self.cpu_request = cpu_request
        self.optimization_interval = optimization_interval
        self.diurnal_len_in_min = diurnal_len_in_min

    def get_workload_ue_weight(self):
        """
        Return the importance (or weight) of under-estimation (UE) for objective formulation.
        Weight is computed based on workload characterization into 
            (a) periodic vs non-periodic
            (b) spread measure
            (c) workload priority
        
        Args
        ----------
        samples : numpy array
            resource utilization timeseries
        priority: str
            workload priority being 'low', 'mid' or 'high'
            cpu_request : float
                Absolute value of requested CPU resource
            
        Returns
        -------
        weight: float
            Workload Under-estimation weight
        isperiodic: bool
            True if the workload has 24 hour periodicity
        spread: float
            The measure of spread of the workload
        """
        
        LOGGER.debug("In get_workload_ue_weight()")
        
        samples = np.asarray(self.samples) / self.cpu_request # Normalize utilization between [0,1]
        samples = curate_samples(samples, self.optimization_interval) # Remove outliers and use a multiple of last 'n' days
        
        # Compute periodicity
        isperiodic = self.get_daily_periodicity()

        # Compute spread
        spread = self.get_spread_measure()
        
        if self.priority == self.HIGH_PRIORITY:
            priority_impact = self.PRIORITY_IMPACT_HIGH
        elif self.priority == self.MID_PRIORITY:
            priority_impact = self.PRIORITY_IMPACT_MID
        elif self.priority == self.LOW_PRIORITY:
            priority_impact = self.PRIORITY_IMPACT_LOW
        elif self.priority == None: # Not specified, treat as highest priority
            priority_impact = self.PRIORITY_IMPACT_HIGH
        else:
            LOGGER.warning("WARNING: Incorrect priority specified. Using highest priority")
            priority_impact = self.PRIORITY_IMPACT_HIGH

        period_impact = self.PERIODICITY_IMPACT * isperiodic
        spread_impact = self.SPREAD_IMPACT * spread
        weight = self.REC_UPPER_VALUE - period_impact - spread_impact - priority_impact
        return [weight, isperiodic, spread]

    def get_spread_measure(self):
        """Calculate spread (normalized standard deviation) of samples.
        """
        std = np.std(self.samples)
        norm_std = std / self.MAX_STD  # norm_std would be between 0 and 1
        return norm_std

    def get_daily_periodicity(self):
        """Calculate periodicity and return True if the workload has daily periodicity
        
        Returns
        -------
        flag: bool
            True if the samples show 24 hour periodicity
        """

        # standardise samples
        samples = (self.samples-np.mean(self.samples))/np.std(self.samples)

        # compute power spectrum of truncated requests array, x
        y = np.abs(np.fft.rfft(samples).real)**self.POWER_SPECTRUM_EXPONENT
        # get frequencies 
        freqs = np.fft.rfftfreq(len(samples), self.FFT_SAMPLING_SPACE)
        
        # normalise power spectrum
        y /= y.sum()    

        # Compute top 5 power components and corresponding frequencies
        top_idxs = np.argpartition(y, self.POWER_COMPONENTS_ACCOUNTED)[self.POWER_COMPONENTS_ACCOUNTED:]
        top_n_periodicity_power = y[top_idxs]
        top_n_freqs = freqs[top_idxs]

        if ((1.0 / self.diurnal_len_in_min) in top_n_freqs): # Check whether sample periodicity=diurnal_len_in_min exists in top-5
            # Find the power of the daily periodic component
            idx_dayperiod = np.where(top_n_freqs == (1.0 / self.diurnal_len_in_min))
            if top_n_periodicity_power[idx_dayperiod] > self.PERIOD_POWER_THR: # Check the power/extent of periodicity
                return True
        return False

