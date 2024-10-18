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
# Author: Gingfung Yeung
# Date: 2024-10-18
# Description: This file is used to deploy Redis VPA objective  test vpa

#script to start all the components of predictive VPA.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
echo "Deploying Redis VPA object"
kubectl apply -f $SCRIPT_DIR/../manifests/vpa_objects/redis_vpa.yaml
status=$?
[ $status -eq 0 ] && echo "Deployed successfully" || echo "Deployment failed.. `exit 1`"
