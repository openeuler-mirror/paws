# Copyright (c) Huawei Technologies Co., Ltd. 2023. All rights reserved.
# paws licensed under the Mulan PSL v2.
# You can use this software according to the terms and conditions of the Mulan PSL v2.
# You may obtain a copy of Mulan PSL v2 at:
#     http://license.coscl.org.cn/MulanPSL2
# THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
# PURPOSE.
# See the Mulan PSL v2 for more details.
# Author: Wei Wei
# Create: 2023-09-18

ARG ARCH
FROM golang:1.20.1

WORKDIR /go/src/gitee.com/openeuler/paws/scheduler
COPY . .
ARG ARCH
ARG RELEASE_VERSION
RUN RELEASE_VERSION=${RELEASE_VERSION} make build-scheduler.$ARCH

FROM $ARCH/alpine:3.16

COPY --from=0 /go/src/gitee.com/openeuler/paws/scheduler/bin/paws-scheduler /bin/paws-scheduler

WORKDIR /bin
CMD ["paws-scheduler"]

