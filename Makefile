# Copyright 2019 The Kubernetes Authors.
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

# Specify whether this repo is build locally or not, default values is '1';
# If set to 1, then you need to also set 'DOCKER_USERNAME' and 'DOCKER_PASSWORD'
# environment variables before build the repo.
BUILD_LOCALLY ?= 1

# The namespace that the test operator will be deployed in
NAMESPACE=ibm-common-services

# Image URL to use all building/pushing image targets;
# Use your own docker registry and image name for dev/test by overridding the
# IMAGE_REPO, IMAGE_NAME and RELEASE_TAG environment variable.
IMAGE_REPO ?= hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com/ibmcom
IMAGE_NAME ?= ibm-healthcheck-operator


# Github host to use for checking the source tree;
# Override this variable ue with your own value if you're working on forked repo.
GIT_HOST ?= github.com/IBM

PWD := $(shell pwd)
BASE_DIR := $(shell basename $(PWD))

# Keep an existing GOPATH, make a private one if it is undefined
GOPATH_DEFAULT := $(PWD)/.go
export GOPATH ?= $(GOPATH_DEFAULT)
GOBIN_DEFAULT := $(GOPATH)/bin
export GOBIN ?= $(GOBIN_DEFAULT)
TESTARGS_DEFAULT := "-v"
export TESTARGS ?= $(TESTARGS_DEFAULT)
DEST := $(GOPATH)/src/$(GIT_HOST)/$(BASE_DIR)
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
RELEASE_VERSION ?= $(shell cat ./version/version.go | grep "Version =" | awk '{ print $$3}' | tr -d '"')
CSV_VERSION ?= $(RELEASE_VERSION)

LOCAL_OS := $(shell uname)
ifeq ($(LOCAL_OS),Linux)
    TARGET_OS ?= linux
    XARGS_FLAGS="-r"
else ifeq ($(LOCAL_OS),Darwin)
    TARGET_OS ?= darwin
    XARGS_FLAGS=
else
    $(error "This system's OS $(LOCAL_OS) isn't recognized/supported")
endif

ARCH := $(shell uname -m)
LOCAL_ARCH := amd64
ifeq ($(ARCH),x86_64)
    LOCAL_ARCH=amd64
else ifeq ($(ARCH),ppc64le)
    LOCAL_ARCH=ppc64le
else ifeq ($(ARCH),s390x)
    LOCAL_ARCH=s390x
else
    $(error "This system's ARCH $(ARCH) isn't recognized/supported")
endif

all: fmt check test coverage build images

ifeq (,$(wildcard go.mod))
ifneq ("$(realpath $(DEST))", "$(realpath $(PWD))")
    $(error Please run 'make' from $(DEST). Current directory is $(PWD))
endif
endif

include common/Makefile.common.mk

############################################################
# configure githooks
############################################################
configure-githooks: ## Configure githooks
	- git config core.hooksPath common/scripts/.githooks

############################################################
# csv section env -> push-csv : CSV_VERSION   bump-up : NEW_CSV_VERSION
############################################################
push-csv: ## Push CSV package to the catalog
	@echo "push-csv ${CSV_VERSION} ..."
	@common/scripts/push-csv.sh ${CSV_VERSION}

bump-up-csv: ## Bump up CSV version
	@echo "bump-up-csv ${BASE_DIR} ${NEW_CSV_VERSION} ..."
	@common/scripts/bump-up-csv.sh "${BASE_DIR}" "${NEW_CSV_VERSION}"

############################################################
# work section
############################################################
$(GOBIN):
	@echo "create gobin"
	@mkdir -p $(GOBIN)

work: $(GOBIN)

############################################################
# format section
############################################################
# All available format: format-go format-python
# Default value will run all formats, override these make target with your requirements:
#    eg: fmt: format-go format-protos
fmt: format-go format-python

############################################################
# check section
############################################################
check: lint
# All available linters: lint-dockerfiles lint-scripts lint-yaml lint-copyright-banner lint-go lint-python lint-helm lint-markdown
# Default value will run all linters, override these make target with your requirements:
#    eg: lint: lint-go lint-yaml
# The MARKDOWN_LINT_WHITELIST variable can be set with comma separated urls you want to whitelist
lint: lint-all

############################################################
# test section
############################################################
test: ## Run unit test
	@echo "Running the tests for $(IMAGE_NAME) on $(LOCAL_ARCH)..."
	@go test $(TESTARGS) ./pkg/controller/...

############################################################
# coverage section
############################################################
coverage:
	@common/scripts/codecov.sh ${BUILD_LOCALLY}

############################################################
# build section
############################################################
build: 
	@echo "Building the $(IMAGE_NAME) binary for $(LOCAL_ARCH)..."
	@GOARCH=$(LOCAL_ARCH) common/scripts/gobuild.sh build/_output/bin/$(IMAGE_NAME) ./cmd/manager
	@strip $(STRIP_FLAGS) build/_output/bin/$(IMAGE_NAME)

############################################################
# image section
############################################################
ifeq ($(BUILD_LOCALLY),0)
    export CONFIG_DOCKER_TARGET = config-docker
endif

build-push-image: build-image push-image

build-image: build
	@echo "Building the $(IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	@docker build -t $(IMAGE_REPO)/$(IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION) -f build/Dockerfile .

push-image: $(CONFIG_DOCKER_TARGET) build-image
	@echo "Pushing the $(IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	@docker push $(IMAGE_REPO)/$(IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION)

############################################################
# multiarch-image section
############################################################
multiarch-image: $(CONFIG_DOCKER_TARGET)
	@common/scripts/multiarch_image.sh $(IMAGE_REPO) $(IMAGE_NAME) $(VERSION) $(RELEASE_VERSION)

############################################################
# application section
############################################################

install: ## Install all resources (CR/CRD's, RBCA and Operator)
	@echo ....... Set environment variables ......
	- export DEPLOY_DIR=deploy/crds
	- export WATCH_NAMESPACE=${NAMESPACE}
	@echo ....... Applying CRDS and Operator .......
	- for crd in $(shell ls deploy/crds/*_crd.yaml); do kubectl apply -f $${crd}; done
	@echo ....... Applying RBAC .......
	- kubectl apply -f deploy/service_account.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/role.yaml -n ${NAMESPACE}
	- cat deploy/role_binding.yaml | sed -e "s|namespace: ibm-healthcheck-operator|namespace: ${NAMESPACE}|g" | kubectl apply -f -
	@echo ....... Applying Operator .......
	- cat deploy/olm-catalog/${BASE_DIR}/${CSV_VERSION}/${BASE_DIR}.v${CSV_VERSION}.clusterserviceversion.yaml \
		| sed -e "s|image: quay.io/opencloudio/ibm-healthcheck-operator.*|image: $(IMAGE_REPO)/$(IMAGE_NAME)-$(LOCAL_ARCH):$(RELEASE_VERSION)|g" \
		| sed -e "s|namespace: placeholder|namespace: ${NAMESPACE}|g" | kubectl apply -f -
	@echo ....... Creating the Instance .......
	- for cr in $(shell ls deploy/crds/*_cr.yaml); do kubectl apply -f $${cr} -n ${NAMESPACE}; done

uninstall: ## Uninstall all that all performed in the $ make install
	@echo ....... Uninstalling .......
	@echo ....... Deleting CR .......
	- for cr in $(shell ls deploy/crds/*_cr.yaml); do kubectl delete -f $${cr} -n ${NAMESPACE}; done
	@echo ....... Deleting Operator .......
	- cat deploy/olm-catalog/${BASE_DIR}/${CSV_VERSION}/${BASE_DIR}.v${CSV_VERSION}.clusterserviceversion.yaml | sed -e "s|namespace: placeholder|namespace: ${NAMESPACE}|g" | kubectl delete -f -
	@echo ....... Deleting CRDs.......
	- for crd in $(shell ls deploy/crds/*_crd.yaml); do kubectl delete -f $${crd}; done
	@echo ....... Deleting Rules and Service Account .......
	- cat deploy/role_binding.yaml | sed -e "s|namespace: ibm-healthcheck-operator|namespace: ${NAMESPACE}|g" | kubectl delete -f -
	- kubectl delete -f deploy/service_account.yaml -n ${NAMESPACE}
	- kubectl delete -f deploy/role.yaml -n ${NAMESPACE}

############################################################
# clean section
############################################################
clean:
	@rm -rf build/_output

.PHONY: all work fmt check coverage lint test build image images multiarch-image clean
