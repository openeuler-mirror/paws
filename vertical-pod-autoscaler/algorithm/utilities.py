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
# Description: Helper functions for algorithm package

import os
import errno
import numpy as np

# Local imports
from utils.utilities import setup_logging

LOGGER = setup_logging(__file__)

def list_dir(root):
    result = []
    for path, dirs, files in os.walk(root):
        for name in files:
            if not name.startswith("~") and not name.lower().endswith((".xlsx")):
                result.append(os.path.join(path, name))
    return result

def custom_log(base, x):
    return (np.log(x) / np.log(base))

def silentremove(filename):
    try:
        os.remove(filename)
    except OSError as e:
        if e.errno != errno.ENOENT:
            LOGGER.error("Cannot remove file", exc_info=True)
            raise # re-raise exception if a different error occurred
        
def curate_samples(samples, optimization_interval):
    # Replace NaNs with nearest real values
    mask = np.isnan(samples)
    samples[mask] = np.interp(np.flatnonzero(mask), np.flatnonzero(~mask), samples[~mask])

    # Remove extreme values
    upper = np.percentile(samples, 99.9)
    lower = np.percentile(samples, 0.1)
    samples[samples > upper] = upper
    samples[samples < lower] = lower
        
    # Only use last 'n' intervals and remove additional points
    if len(samples) % optimization_interval != 0:
        LOGGER.warning(f"Length of samples  for curation:{len(samples)} is > OPT interval:{optimization_interval}")
        samples = samples[len(samples) % optimization_interval : ]

    return samples