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
from algorithm.linear_regression_model import LinearRegressionModel
from utils.utilities import setup_logging

LOGGER = setup_logging(__file__)

class MLPredictor():
    def __init__(self, target_all_days, forecast_horizon=1, model_type = 'LinearRegression'):
        self.MIN_SAMPLES_FOR_ML = 3
        self.target_all_days = np.asarray(target_all_days)
        self.forecast_horizon = forecast_horizon
        self.model_type = model_type

    def predictor_runner(self):
        LOGGER.debug(f"Length of samples input to prediction_runner: {len(self.target_all_days)}")

        # Call the ML API to predict the next target value
        if len(self.target_all_days) <= self.MIN_SAMPLES_FOR_ML: # TODO: For auto mode, increase this value
            target = np.amax(self.target_all_days)
            m, c = 0, 0
        else:
            if self.model_type == 'LinearRegression':
                LOGGER.info("Performing Linear Regression")
                # Train, if not already
                if not self.trained_model_exists():
                    context_size = 1
                    ml_model = LinearRegressionModel(self.target_all_days, context_size, self.forecast_horizon)
                    ml_model.train()
                else:
                    ml_model = self.load_trained_model()
                # Predict
                target, m, c = ml_model.predict() # m,c can be used in future for curve estimation
            else:
                LOGGER.warning("ML Model Type not exist")
                raise NotImplementedError
    
        return target

    def trained_model_exists(self):
        return False # This LR model trains in real time

    def load_trained_model(self):
        return False
