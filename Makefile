# Current Operator version
VERSION ?= 0.0.1
# Default bundle image tag
BUNDLE_IMG ?= samba-operator-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

COMMIT_ID = $(shell git describe --abbrev=40 --always --dirty=+ 2>/dev/null)
GIT_VERSION = $(shell git describe --match='v[0-9]*.[0-9].[0-9]' 2>/dev/null || echo "(unset)")

CONFIG_KUST_DIR:=config/default
CRD_KUST_DIR:=config/crd
MGR_KUST_DIR:=config/manager
KUSTOMIZE_DEFAULT_BASE:=../default

ifneq ($(DEVELOPER),)
CONFIG_KUST_DIR:=config/developer
MGR_KUST_DIR:=config/developer
endif

GO_CMD:=go
GOFMT_CMD:=gofmt
KUBECTL_CMD:=kubectl
BUILDAH_CMD:=buildah

# Image URL to use all building/pushing image targets
TAG ?= latest
IMG ?= quay.io/samba.org/samba-operator:$(TAG)

# Produce CRDs that work on Kubernetes 1.16 or later
CRD_OPTIONS ?= "crd:trivialVersions=true,crdVersions=v1"

CHECK_GOFMT_FLAGS ?= -e -s -l

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell $(GO_CMD) env GOBIN))
GOBIN=$(shell $(GO_CMD) env GOPATH)/bin
else
GOBIN=$(shell $(GO_CMD) env GOBIN)
endif

CONTAINER_BUILD_OPTS ?=
CONTAINER_CMD ?=
ifeq ($(CONTAINER_CMD),)
	CONTAINER_CMD:=$(shell docker version >/dev/null 2>&1 && echo docker)
endif
ifeq ($(CONTAINER_CMD),)
	CONTAINER_CMD:=$(shell podman version >/dev/null 2>&1 && echo podman)
endif
# handle the case where podman is present but is (defaulting) to remote and is
# not not functioning correctly. Example: mac platform but not 'podman machine'
# vms are ready
ifeq ($(CONTAINER_CMD),)
	CONTAINER_CMD:=$(shell podman --version >/dev/null 2>&1 && echo podman)
ifneq ($(CONTAINER_CMD),)
$(warning podman detected but 'podman version' failed. \
	this may mean your podman is set up for remote use, but is not working)
endif
endif

all: manager build-integration-tests

# Run tests
test: generate manifests vet
	hack/test.sh

# Build manager binary
manager: generate build vet

build:
	CGO_ENABLED=0 $(GO_CMD) build -o bin/manager -ldflags "-X main.Version=$(GIT_VERSION) -X main.CommitID=$(COMMIT_ID)"  main.go
.PHONY: build

build-integration-tests:
	$(GO_CMD) test -c -o bin/integration-tests -tags integration ./tests/integration
.PHONY: build-integration-tests

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate vet manifests
	$(GO_CMD) run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build $(CRD_KUST_DIR) | $(KUBECTL_CMD) apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build $(CRD_KUST_DIR) | $(KUBECTL_CMD) delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize set-image
	$(KUSTOMIZE) build $(CONFIG_KUST_DIR) | $(KUBECTL_CMD) apply -f -

delete-deploy: manifests kustomize
	$(KUSTOMIZE) build $(CONFIG_KUST_DIR) | $(KUBECTL_CMD) delete -f -

%/kustomization.yaml: $(KUSTOMIZE)
	mkdir -p $*
	touch $@
	cd $* && $(KUSTOMIZE) edit add base $(KUSTOMIZE_DEFAULT_BASE)

set-image: kustomize $(MGR_KUST_DIR)/kustomization.yaml
	cd $(MGR_KUST_DIR) && $(KUSTOMIZE) edit set image controller=${IMG}
.PHONY: set-image

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=$(CRD_KUST_DIR)/bases

# Run go fmt to reformat code
reformat:
	$(GO_CMD) fmt ./...

# Run go vet against code
vet:
	$(GO_CMD) vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the container image
docker-build: image-build
image-build:
	$(CONTAINER_CMD) build \
		--build-arg=GIT_VERSION="$(GIT_VERSION)" \
		--build-arg=COMMIT_ID="$(COMMIT_ID)" \
		$(CONTAINER_BUILD_OPTS) $(CONTAINER_BUILD_OPTS) . -t ${IMG}

.PHONY: image-build-buildah
image-build-buildah: build
	cn=$$($(BUILDAH_CMD) from registry.access.redhat.com/ubi8/ubi-minimal:latest) && \
	$(BUILDAH_CMD) copy $$cn bin/manager /manager && \
	$(BUILDAH_CMD) config --cmd='[]' $$cn && \
	$(BUILDAH_CMD) config --entrypoint='["/manager"]' $$cn && \
	$(BUILDAH_CMD) commit $$cn ${IMG}

# Push the container image
docker-push: container-push
container-push:
	$(CONTAINER_CMD) push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell command -v controller-gen ;))
	@echo "controller-gen not found in PATH, checking in GOBIN ($(GOBIN))..."
ifeq (, $(shell command -v $(GOBIN)/controller-gen ;))
	$(GO_CMD) install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.2
	@echo "controller-gen installed in GOBIN"
endif
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell command -v controller-gen ;)
endif

kustomize:
ifeq (, $(shell command -v kustomize ;))
	@echo "kustomize not found in PATH, checking in GOBIN ($(GOBIN))..."
ifeq (, $(shell command -v $(GOBIN)/kustomize ;))
	$(GO_CMD) install sigs.k8s.io/kustomize/kustomize/v4@v4.3.0
	@echo "kustomize installed in GOBIN"
endif
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell command -v kustomize ;)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests
	@echo "This rule is currently disabled. It is retained for reference only."
	@false
	# See vcs history for how this could be restored or adapted in the future.

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	@echo "This rule is currently disabled. It is retained for reference only."
	@false
	# See vcs history for how this could be restored or adapted in the future.

.PHONY: check check-revive check-format

check: check-revive check-format vet

check-format:
	! $(GOFMT_CMD) $(CHECK_GOFMT_FLAGS) . | sed 's,^,formatting error: ,' | grep 'go$$'

check-revive: revive
	# revive's checks are configured using .revive.toml
	# See: https://github.com/mgechev/revive
	$(REVIVE) -config .revive.toml $$($(GO_CMD) list ./... | grep -v /vendor/)

.PHONY: revive
revive:
ifeq (, $(shell command -v revive ;))
	@echo "revive not found in PATH, checking in GOBIN ($(GOBIN))..."
ifeq (, $(shell command -v $(GOBIN)/revive ;))
	$(GO_CMD) install github.com/mgechev/revive@latest
	@echo "revive installed in GOBIN"
else
	@echo "revive found in GOBIN"
endif
REVIVE=$(shell command -v $(GOBIN)/revive ;)
else
	@echo "revive found in PATH"
REVIVE=$(shell command -v revive ;)
endif
