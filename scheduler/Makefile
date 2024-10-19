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


ARCHS = amd64 arm64
COMMONENVVAR = GOOS=$(shell uname -s | tr A-Z a-z)
BUILDENVVAR = CGO_ENABLED=0

# Local registry and images
LOCAL_REGISTRY ?= localhost:5000
LOCAL_SCHEDULER_IMAGE ?= paws-scheduler:latest
LOCAL_CONTROLLER_IMAGE ?= paws-controller:latest

# Release registry and versioning
RELEASE_REGISTRY ?= docker.io/sirlabb
RELEASE_VERSION ?= v$(shell date +%Y%m%d)-v0.0.1
RELEASE_SCHEDULER_IMAGE := paws-scheduler:$(RELEASE_VERSION)
RELEASE_CONTROLLER_IMAGE := paws-controller:$(RELEASE_VERSION)

# Extract version from RELEASE_VERSION
VERSION = $(shell echo $(RELEASE_VERSION) | awk -F - '{print $$2}')

# Paths
SCHEDULER_PATH = cmd/paws-scheduler/main.go
CONTROLLER_PATH = cmd/paws-controller/main.go

.PHONY: all
all: build

# Build targets
.PHONY: build
build: build-scheduler build-controller

.PHONY: build.amd64
build.amd64: build-scheduler.amd64 build-controller.amd64

.PHONY: build.arm64v8
build.arm64v8: build-scheduler.arm64v8 build-controller.arm64v8

# Scheduler build
.PHONY: build-scheduler
build-scheduler: $(ARCHS:%=build-scheduler.%)

.PHONY: build-scheduler.%
build-scheduler.%:
	$(COMMONENVVAR) $(BUILDENVVAR) GOARCH=$* go build -ldflags '-X k8s.io/component-base/version.gitVersion=$(VERSION) -w' -o bin/paws-scheduler $(SCHEDULER_PATH)

# Controller build
.PHONY: build-controller
build-controller: $(ARCHS:%=build-controller.%)

.PHONY: build-controller.%
build-controller.%:
	$(COMMONENVVAR) $(BUILDENVVAR) GOARCH=$* go build -ldflags '-w' -o bin/controller $(CONTROLLER_PATH)

# Local Docker build
.PHONY: local-image
local-image: clean
	docker build -f ./build/paws-scheduler/Dockerfile --build-arg ARCH="amd64" --build-arg RELEASE_VERSION="$(RELEASE_VERSION)" -t $(LOCAL_REGISTRY)/$(LOCAL_SCHEDULER_IMAGE) .
	docker build -f ./build/paws-controller/Dockerfile --build-arg ARCH="amd64" -t $(LOCAL_REGISTRY)/$(LOCAL_CONTROLLER_IMAGE) .

# Release Docker build
.PHONY: release-image
release-image: clean $(ARCHS:%=release-image.%)

.PHONY: release-image.%
release-image.%:
	docker build -f ./build/paws-scheduler/Dockerfile --build-arg ARCH=$* --build-arg RELEASE_VERSION="$(RELEASE_VERSION)" -t $(RELEASE_REGISTRY)/$(RELEASE_SCHEDULER_IMAGE)-$* .
	docker build -f ./build/paws-controller/Dockerfile --build-arg ARCH=$* -t $(RELEASE_REGISTRY)/$(RELEASE_CONTROLLER_IMAGE)-$* .

# Push release images and create manifest
.PHONY: push-release-images
push-release-images: release-image
	for arch in $(ARCHS); do \
		docker push $(RELEASE_REGISTRY)/$(RELEASE_SCHEDULER_IMAGE)-$${arch} ;\
		docker push $(RELEASE_REGISTRY)/$(RELEASE_CONTROLLER_IMAGE)-$${arch} ;\
	done
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest create $(RELEASE_REGISTRY)/$(RELEASE_SCHEDULER_IMAGE) $(addprefix --amend $(RELEASE_REGISTRY)/$(RELEASE_SCHEDULER_IMAGE)-, $(ARCHS))
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest create $(RELEASE_REGISTRY)/$(RELEASE_CONTROLLER_IMAGE) $(addprefix --amend $(RELEASE_REGISTRY)/$(RELEASE_CONTROLLER_IMAGE)-, $(ARCHS))
	for arch in $(ARCHS); do \
		DOCKER_CLI_EXPERIMENTAL=enabled docker manifest annotate --arch $${arch} $(RELEASE_REGISTRY)/$(RELEASE_SCHEDULER_IMAGE) $(RELEASE_REGISTRY)/$(RELEASE_SCHEDULER_IMAGE)-$${arch} ;\
		DOCKER_CLI_EXPERIMENTAL=enabled docker manifest annotate --arch $${arch} $(RELEASE_REGISTRY)/$(RELEASE_CONTROLLER_IMAGE) $(RELEASE_REGISTRY)/$(RELEASE_CONTROLLER_IMAGE)-$${arch} ;\
	done
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest push $(RELEASE_REGISTRY)/$(RELEASE_SCHEDULER_IMAGE)
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest push $(RELEASE_REGISTRY)/$(RELEASE_CONTROLLER_IMAGE)

# Helper tasks
.PHONY: update-vendor
update-vendor:
	hack/update-vendor.sh

.PHONY: verify
verify:
	hack/verify-gofmt.sh

.PHONY: clean
clean:
	rm -rf ./bin

