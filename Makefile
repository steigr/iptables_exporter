# Copyright 2018 The Prometheus Authors
# Copyright 2018 RetailNext, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# RN: Removed docker-related targets

# Ensure GOBIN is not set during build so that promu is installed to the correct path
unexport GOBIN

GO           ?= go
GOFMT        ?= $(GO)fmt
FIRST_GOPATH := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))
PROMU        := $(FIRST_GOPATH)/bin/promu
STATICCHECK  := $(FIRST_GOPATH)/bin/staticcheck
GOVENDOR     := $(FIRST_GOPATH)/bin/govendor
pkgs          = ./...

PREFIX                  ?= $(shell pwd)
BIN_DIR                 ?= $(shell pwd)

DOCKER_IMAGE ?= quay.io/steigr/iptables-exporter
DOCKER_TAG   ?= v$(shell cat VERSION)

all: style staticcheck unused build test

style:
	@echo ">> checking code style"
	! $(GOFMT) -d $$(find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

check_license:
	@echo ">> checking license header"
	@licRes=$$(for file in $$(find . -type f -iname '*.go' ! -path './vendor/*') ; do \
               awk 'NR<=3' $$file | grep -Eq "(Copyright|generated|GENERATED)" || echo $$file; \
       done); \
       if [ -n "$${licRes}" ]; then \
               echo "license header checking failed:"; echo "$${licRes}"; \
               exit 1; \
       fi

test-short:
	@echo ">> running short tests"
	$(GO) test -short $(pkgs)

test:
	@echo ">> running all tests"
	$(GO) test -race $(pkgs)

format:
	@echo ">> formatting code"
	$(GO) fmt $(pkgs)

vet:
	@echo ">> vetting code"
	$(GO) vet $(pkgs)

staticcheck: $(STATICCHECK)
	@echo ">> running staticcheck"
	$(STATICCHECK) $(pkgs)

unused: $(GOVENDOR)
	@echo ">> running check for unused packages"
	@$(GOVENDOR) list +unused | grep . && exit 1 || echo 'No unused packages'

build: promu
	@echo ">> building binaries"
	$(PROMU) build --prefix $(PREFIX)

tarball: promu
	@echo ">> building release tarball"
	$(PROMU) tarball --prefix $(PREFIX) $(BIN_DIR)

promu:
	GOOS= GOARCH= $(GO) get -u github.com/prometheus/promu

$(FIRST_GOPATH)/bin/staticcheck:
	GOOS= GOARCH= $(GO) get -u honnef.co/go/tools/cmd/staticcheck

$(FIRST_GOPATH)/bin/govendor:
	GOOS= GOARCH= $(GO) get -u github.com/kardianos/govendor

docker-image:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

.PHONY: all style check_license format build test vet assets tarball promu staticcheck $(FIRST_GOPATH)/bin/staticcheck govendor $(FIRST_GOPATH)/bin/govendor
