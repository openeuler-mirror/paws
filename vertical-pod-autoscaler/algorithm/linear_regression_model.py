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

class LinearRegressionModel:
    """A simple Linear Regression based model for predicting future values

    Attributes
    ----
        ts_samples : numpy array
            Timeseries (of past recommendation values)
        forecast_horizon : int
            Number of future timestamps to be predicted

    Methods
    ----
        train(self)
        predict(self)
    """
    def __init__(self, ts_samples: np.ndarray, forecast_horizon: int):
        """
        Initializes the linear regression model for time series prediction.

        Parameters:
        ----
        ts_samples: numpy array
            The time series data (samples).
        forecast_horizon: int
            Number of future time steps to predict.
        """
        self.ts_samples = np.asarray(ts_samples)  # Ensure it's a numpy array
        self.forecast_horizon = forecast_horizon
        self.model = None

    def train(self):
        """
        Trains the linear regression model on the time series samples.
        """
        # Create features (X) as time steps and target (y) as the time series values
        X = np.arange(len(self.ts_samples)).reshape(-1, 1)  # Reshape to 2D
        y = self.ts_samples

        # Train the linear regression model
        self.model = LinearRegression().fit(X, y)

        # Log the model's performance and coefficients
        score = self.model.score(X, y)
        coef = self.model.coef_[0]
        intercept = self.model.intercept_
        LOGGER.debug(f"Model score: {score:.4f}, Coefficient: {coef:.4f}, Intercept: {intercept:.4f}")

    def predict(self) -> list:
        """
        Predicts future values based on the trained linear regression model.

        Returns:
        ----
        list
            Predicted future values, the coefficient, and intercept.
        """
        if self.model is None:
            raise ValueError("The model has not been trained yet. Call train() before predict().")

        # Prepare future time steps as features for prediction
        X_test = np.arange(len(self.ts_samples), len(self.ts_samples) + self.forecast_horizon).reshape(-1, 1)

        # Predict the future values
        y_pred = self.model.predict(X_test)

        # Return the predicted values along with model parameters
        return [y_pred.tolist(), self.model.coef_[0], self.model.intercept_]

