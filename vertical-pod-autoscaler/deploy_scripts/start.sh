#!/bin/bash

# Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
# PAWS licensed under the Mulan PSL v2.
# You can use this software according to the terms and conditions of the Mulan PSL v2.
# You may obtain a copy of Mulan PSL v2 at:
#     http://license.coscl.org.cn/MulanPSL2
# THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
# PURPOSE.
# See the Mulan PSL v2 for more details.
# Author: Wei Wei; Gingfung Yeung
# Date: 2024-10-18
# Description: This file is used for paws recommender deployment

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
echo "Installing Recommender"
kubectl apply -f $SCRIPT_DIR/../manifests/core/recommender-deployment.yaml
if [ $? -ne 0 ]; then
  echo "Command 1 failed. Exiting..."
  exit 1
fi
echo "Deployed successfully"
echo "Installing priority classes"
kubectl apply -f $SCRIPT_DIR/../manifests/priority-classes/
if [ $? -ne 0 ]; then
  echo "Command 2 failed. Exiting..."
  exit 1
fi

echo "Completed successfully"
