# Copyright (c) 2018 Intel Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.DEFAULT_GOAL := default
SHELL := /usr/bin/env bash

BINDIR = $(CURDIR)/bin
BUILDDIR = $(CURDIR)/build

# Tools
GOLANGCI_LINT = $(BINDIR)/golangci-lint
GOLANGCI_LINT_VER = v2.7.2
GOLANGCI_LINT_TIMEOUT ?= 10m
export GOLANGCI_LINT_CACHE = $(BUILDDIR)/.cache

$(BINDIR):
	@mkdir -p $@

$(BUILDDIR):
	@mkdir -p $@

$(GOLANGCI_LINT): | $(BINDIR) ; $(info  installing golangci-lint...)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VER))

.PHONY: lint
lint: | $(GOLANGCI_LINT) ; $(info  running golangci-lint...) ## Run golangci-lint
	$(GOLANGCI_LINT) run --timeout=$(GOLANGCI_LINT_TIMEOUT)

default :
	bash scripts/build.sh

image :
	scripts/build-image.sh

.PHONY: test
test :
	scripts/test.sh

vendor :
	go mod tidy && go mod vendor

e2e:
	source scripts/e2e_get_tools.sh && scripts/e2e_setup_cluster.sh
	go test -timeout 40m -v ./test/e2e/...

e2e-clean:
	source scripts/e2e_get_tools.sh && scripts/e2e_teardown_cluster.sh
	scripts/e2e_cleanup.sh

deps-update: ; $(info  Updating dependencies...) @ ## Update dependencies
	@go mod tidy

# go-install-tool will 'go install' any package $2 and install it to $1.
define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
echo "Downloading $(2)" ;\
GOBIN=$(BINDIR) go install -mod=mod $(2) ;\
}
endef
