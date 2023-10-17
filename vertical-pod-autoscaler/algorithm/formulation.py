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
# Description: Core MILP formulation

import numpy as np

class ProblemFormulation():
    """Formulation of objective function and constraints
    
    Attributes
    ----------
        samples : numpy array
            resource utilization timeseries
        throttles: numpy array
            Expects throttles to be percentage time a pod is throttled
        ue_weight : float
            Importance or weight of Under Estimation
            
    Methods
    -------
        objective(x)
        constraint_PE(x)
        constraint_PE(x)
    """
    
    def __init__(self, samples, throttle_samples, ue_weight):
        self.samples = samples
        self.throttle_samples = throttle_samples
        self.ue_weight = ue_weight
        
    def objective(self, x):
        """Objective function
        
        Args
        ----
            x : float
                parameter to optimize
                
        Returns
        -------
            float
            Weighted average of under-estimation and over-estimation
        """

        under_pred = self.samples-x
        ue_vec = (under_pred >= 0) * under_pred
        oe_vec = (under_pred < 0) * (-1 * under_pred)
        error = (self.ue_weight*ue_vec) +  ((1-self.ue_weight)*oe_vec)
        return np.sum(error)

    def constraint_PE(self,x):
        """Constraint on percentage error being above a threshold
        
        Args
        ----------
            x : float
                parameter to optimize
                
        Returns
        -------
            percent_erro : float
            Percentage error in under-estimation
        """
        over_predict = x - self.samples
        percent_error = np.average(np.abs(over_predict[over_predict < 0]) / self.samples[over_predict < 0])
        percent_error *= 100
        return percent_error

    def constraint_throttles(self,x):
        """Constraint on percentage error being above a threshold
        
        Args
        ----------
            x : float
                parameter to optimize
                
        Returns
        -------
            percent_erro : float
            Average throttles in the period under consideration
        """
        return np.average(self.throttle_samples)
