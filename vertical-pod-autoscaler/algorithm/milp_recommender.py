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
# Create: 2023-09-10
# Description: Wrapper API for MILP module

import numpy as np
from scipy.optimize import (BFGS, SR1, Bounds, NonlinearConstraint, minimize)

# Local imports
from algorithm.formulation import ProblemFormulation
from algorithm.utilities import curate_samples
from utils.utilities import setup_logging

LOGGER = setup_logging(__file__)

class MILPRecommender():
    """    
    Attributes
    ----------
        samples : numpy array
            resource utilization timeseries
        throttle_samples : numpy array
            throttle timeseries
        optimization_interval : int
            Length of update or optimizatio interval for auto mode
        ue_weight : float
            Importance or weight of Under Estimation
        cpu_request : float
            Absolute value of requested CPU resource
            
    Methods
    -------
        milp_recommender(self)
        optimizer(self, samples, throttle_samples)

    """

    INCREMENT_INTERVAL_BY = 1
    TARGET_PERCENTILE = 98
    VERBOSE = 0 # 0,1,2,3
    PE_LOWER_BOUND = 0
    PE_UPPER_BOUND = 5 # Average percentage error per-day threshold
    PE_UPPER_BOUND_RELAXED = 10
    MAX_ACCEPTABLE_THROTTLE = 0.1 # 10 percent throttles
    OPT_SOLUTION_MIN = 0.05 # This is in percentage (relative to CPU request)
    OPT_SOLUTION_MAX = 2.0 # This is in percentage. Change this if samples are not normalized to [0,1]
    NUM_RETRIES = 2
    
    def __init__(self, samples, throttle_samples, optimization_interval, ue_weight=0.99, cpu_request=1.0):
        self.samples = samples
        self.throttle_samples = throttle_samples
        self.ue_weight = ue_weight
        self.cpu_request = cpu_request
        self.optimization_interval = optimization_interval

    def milp_recommender(self):
        """Compute the CPU utilization recommendation for rightsizing
                
        Returns
        -------
            target_all_days : float
                CPU recommendation for all input days or intervals
        """
    
        LOGGER.debug("In milp_recommender()")
    
        samples = np.asarray(self.samples) / self.cpu_request # Normalize utilization between [0,1]
        throttle_samples = np.asarray(self.throttle_samples)

        if len(samples) < self.optimization_interval:
            LOGGER.warning(f"Short lived pod: samples less than {self.optimization_interval}")
            target = np.percentile(samples, self.TARGET_PERCENTILE)
            target_all_days = [target * self.cpu_request] # Reconvert to absolute value of CPU cores
        else:            
             # Remove outliers and return a multiple of last 'n' intervals
            samples = curate_samples(samples, self.optimization_interval)
            throttle_samples = curate_samples(throttle_samples, self.optimization_interval)
            n_past_intervals = int(len(samples) / self.optimization_interval)

            LOGGER.debug(f"Number of intervals to optimize by MILP: {n_past_intervals}")
            target_all_days = [] # Compute target for each day independently
            for past_interval_id in range(n_past_intervals):
                samples_in_interval = samples[past_interval_id * self.optimization_interval : 
                                        (past_interval_id + self.INCREMENT_INTERVAL_BY) * self.optimization_interval]
                samples_in_day_throttle = throttle_samples[past_interval_id * self.optimization_interval :
                                        (past_interval_id + self.INCREMENT_INTERVAL_BY) * self.optimization_interval]
                target = self.optimizer(samples_in_interval, samples_in_day_throttle)
                target_all_days.append(target * self.cpu_request) # Reconvert to absolute value of CPU cores
        
        return target_all_days

    def optimizer(self, samples, throttle_samples):
        """Wrapper API to compute the optimization objective
        
        Args
        ----
            samples : numpy array
                resource utilization timeseries
            throttle_samples : numpy array
                throttle timeseries
                
        Returns
        -------
            solution : float 
                target value of resource recommendation
        """
        
        initial_guess = np.percentile(samples, 95) # Use a reasonable starting point helps since scipy is not a Global solver

        # Define the problem formulation
        p = ProblemFormulation(samples, throttle_samples, self.ue_weight)
        bounds = Bounds([self.OPT_SOLUTION_MIN], [self.OPT_SOLUTION_MAX])
        nonlinear_constraint = NonlinearConstraint(p.constraint_PE, self.PE_LOWER_BOUND, self.PE_UPPER_BOUND, jac='2-point', hess=BFGS(), keep_feasible=False)
        nonlinear_constraint_relaxed = NonlinearConstraint(p.constraint_PE, self.PE_LOWER_BOUND, self.PE_UPPER_BOUND_RELAXED, jac='2-point', hess=BFGS(), keep_feasible=False)
        
        # perform the l-bfgs-b OR 'trust-constr' algorithm search
        method = 'trust-constr' # 'L-BFGS-B' # 'trust-constr'
        c = [nonlinear_constraint]
        exception_count = 0
        while exception_count < self.NUM_RETRIES: # Try optimizer twice before reverting to simple solution
            try:
                result = minimize(p.objective,
                                initial_guess, 
                                method=method,
                                jac="2-point",
                                hess=SR1(),
                                constraints=c,
                                options={'verbose': self.VERBOSE},
                                bounds=bounds)
            except Exception as e:
                exception_count += 1
                if exception_count < self.NUM_RETRIES: # If error raised in first attempt
                    LOGGER.warning("Infeasible solution. Trying with the relaxed constraint set.")
                    c = [nonlinear_constraint_relaxed]
                else: # If error raised in second attempt
                    LOGGER.warning("Infeasible solution in %d attempt. Resorting to default percentile." % exception_count)
                    solution = np.percentile(samples, self.TARGET_PERCENTILE)
                    solution =  [solution + (0.2*solution)] # 20% higher than a fixed percentile
                    break
            else:
                # LOGGER.debug('MILP solver successful. Status : %s' % result['message'])
                solution = result['x']
                break
        
        if (solution[0] < self.OPT_SOLUTION_MIN): # Lower bound check
            LOGGER.warning(f"Solution = {solution[0]}, which is smaller than lower bound={self.OPT_SOLUTION_MIN}. \
                           Resorting to default percentile.")
            solution = np.percentile(samples, self.TARGET_PERCENTILE)
            solution =  [solution - (0.5*solution)] # 50% lower than a fixed percentile

        # Runtime feedback mechanism - If system is highly throttled, increase the recommendation value by
        # the percentage of throttled time obtained by metric: container_cpu_cfs_throttled_seconds_total
        if np.average(throttle_samples) > self.MAX_ACCEPTABLE_THROTTLE:
            LOGGER.warning(f"Average throttles per period = {np.average(throttle_samples)}, \
                           which is greater than acceptable threshold of {self.MAX_ACCEPTABLE_THROTTLE}. \
                           Increasing the target recommendation by throttled time percentage!")
            solution =  [solution[0] + (np.average(throttle_samples)*solution[0])] 
        
        return solution[0]
