.RECIPEPREFIX := >
TARGETS := $(shell ls scripts)

DAPPER_IMAGE ?= pasturestack-mount-propagation-dapper:ubuntu26
DAPPER_HOST_ARCH ?= amd64
DOCKER_VERSION ?= 29.5.3
GO_VERSION ?= 1.26.5
DAPPER_SOURCE ?= /go/src/github.com/PastureStack/mount-propagation

.dapper:
>docker build \
>  --build-arg DAPPER_HOST_ARCH=$(DAPPER_HOST_ARCH) \
>  --build-arg DOCKER_VERSION=$(DOCKER_VERSION) \
>  --build-arg GO_VERSION=$(GO_VERSION) \
>  -t $(DAPPER_IMAGE) \
>  -f Dockerfile.dapper .

$(TARGETS): .dapper
>docker run --rm \
>  -v $(CURDIR):$(DAPPER_SOURCE) \
>  -v /var/run/docker.sock:/var/run/docker.sock \
>  -e DAPPER_UID=$$(id -u) \
>  -e DAPPER_GID=$$(id -g) \
>  -e ARCH=$(DAPPER_HOST_ARCH) \
>  -e TAG \
>  -e REPO \
>  -e VERSION_OVERRIDE \
>  $(DAPPER_IMAGE) $@

trash: deps

trash-keep: deps

deps: .dapper
>docker run --rm \
>  -v $(CURDIR):$(DAPPER_SOURCE) \
>  -e DAPPER_UID=$$(id -u) \
>  -e DAPPER_GID=$$(id -g) \
>  -e ARCH=$(DAPPER_HOST_ARCH) \
>  -e TAG \
>  -e REPO \
>  -e VERSION_OVERRIDE \
>  $(DAPPER_IMAGE) /bin/bash -lc 'go mod download'

.DEFAULT_GOAL := ci

.PHONY: .dapper $(TARGETS) trash trash-keep deps
