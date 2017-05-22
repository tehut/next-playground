# Copyright 2017 Heptio Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

TARGET = ksonnet-playground
GOTARGET = github.com/heptio/$(TARGET)
BUILDMNT = /go/src/$(GOTARGET)
REGISTRY ?= gcr.io/heptio-images
VERSION ?= v0.1
TESTARGS ?= -v
IMAGE = $(REGISTRY)/$(BIN)
BUILD_IMAGE ?= golang:1.8-alpine
DOCKER ?= docker
DIR := ${CURDIR}
BUILD = go build -v

local:
	$(BUILD)

all: cbuild container

cbuild:
	$(DOCKER) run --rm -v $(DIR):$(BUILDMNT) -w $(BUILDMNT) $(BUILD_IMAGE) /bin/sh -c '$(BUILD)'

container: cbuild
	$(DOCKER) build -t $(REGISTRY)/$(TARGET):latest -t $(REGISTRY)/$(TARGET):$(VERSION) .

# TODO: Determine tagging mechanics
push:
	docker -- push $(REGISTRY)/$(TARGET)

.PHONY: all local container cbuild push

clean:
	rm -f $(TARGET) $(TESTTARGET)
	$(DOCKER) rmi -f $(REGISTRY)/$(TARGET):latest
	$(DOCKER) rmi -f $(REGISTRY)/$(TARGET):$(VERSION)
