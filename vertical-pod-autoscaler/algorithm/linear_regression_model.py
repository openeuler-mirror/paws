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
# Create: 2023-09-21
# Description: Machine Learning Wrapper API to call different ML models depending on input args

import numpy as np
from sklearn.linear_model import LinearRegression
from utils.utilities import setup_logging

LOGGER = setup_logging(__file__)

class LinearRegressionModel():
    """A simple Linear Regression based model for predicting future values
    
    Attributes
    ----
        ts_samples : numpy array
            timeseries (of past recommendation values)
        forecast_horizon : int
            number of future timestamps to be predicted

    Methods
    ----
        train(self)
        predict(self)
    """
    def __init__(self, ts_samples, context_size, forecast_horizon):
        self.ts_samples = ts_samples
        self.forecast_horizon = forecast_horizon
        self.model = None

    def train(self):
        # Make features for linear regression
        X = np.expand_dims(np.asarray(range(len(self.ts_samples))), axis=1) # Feature is the value of time instant
        y = np.asarray(self.ts_samples) # Target is the resource utilization value at each time instant

        self.model = LinearRegression().fit(X, y)
        LOGGER.debug(self.model.score(X, y), self.model.coef_, self.model.intercept_)
        LOGGER.debug(f"LR model = {self.model}")
    
    def predict(self):
        X_test = list(np.arange(len(self.ts_samples), len(self.ts_samples) + self.forecast_horizon, 1))
        y_pred = self.model.predict(np.array([X_test]))
        return [y_pred[0],  self.model.coef_[0], self.model.intercept_]
