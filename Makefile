VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS ?= -X minefleet.dev/minecraft-gateway/internal/version.Version=$(VERSION) \
           -X minefleet.dev/minecraft-gateway/internal/version.CommitSHA=$(GIT_COMMIT) \
           -X minefleet.dev/minecraft-gateway/internal/version.BuildDate=$(BUILD_DATE)

# Image URL to use all building/pushing image targets
CONTROLLER_IMG ?= minefleet.dev/minecraft-gateway:$(VERSION)
EDGE_IMG ?= minefleet.dev/minecraft-edge:$(VERSION)
NETWORK_IMG ?= minefleet.dev/minecraft-proxy:$(VERSION)
NETWORK_INTEGRATIONS ?= velocity

# Newline used to separate foreach-generated shell commands onto individual lines
define newline


endef

PLATFORMS ?= linux/arm64,linux/amd64

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen proto ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: proto
proto: buf ## Generate Go and Java code from proto files in api/.
	cd api && $(BUF) generate

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet network-test setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# TODO(user): To use a different vendor for e2e tests, modify the setup under 'tests/e2e'.
# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# CertManager is installed by default; skip with:
# - CERT_MANAGER_INSTALL_SKIP=true
KIND_CLUSTER ?= minecraft-gateway-test-e2e

.PHONY: image-load
image-load: controller-image-load edge-image-load network-image-load

.PHONY: controller-image-load
controller-image-load: ## Install the controller image onto a given Kind cluster
	$(KIND) load docker-image ${CONTROLLER_IMG} --name ${KIND_CLUSTER}

.PHONY: edge-image-load
edge-image-load: ## Install the edge image onto a given Kind cluster
	$(KIND) load docker-image ${EDGE_IMG} --name ${KIND_CLUSTER}

.PHONY: network-image-load
network-image-load: ## Install the network integration images onto a given Kind cluster
	$(call foreach-network-integration,network-image-load)

# network-image-load loads a network integration image to a given kind cluster
# $1 - Integration name
define network-image-load
	$(KIND) load docker-image ${NETWORK_IMG}-$(1) --name ${KIND_CLUSTER}
endef

.PHONY: setup-test-e2e
setup-test-e2e: ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND_CLUSTER=$(KIND_CLUSTER) go test ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	$(GOLANGCI_LINT) config verify

##@ Build Controller

.PHONY: controller-build
controller-build: manifests generate fmt vet ## Build manager binary.
	go build -ldflags "$(LDFLAGS)" -o bin/manager cmd/main.go

.PHONY: controller-run
controller-run: manifests generate fmt vet ## Run a controller from your host.
	go run -ldflags "$(LDFLAGS)" ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: controller-docker-build
controller-docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${CONTROLLER_IMG} \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		. -f Dockerfile.controller

.PHONY: controller-docker-push
controller-docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${CONTROLLER_IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make controller-docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
.PHONY: controller-docker-buildx
controller-docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile.controller and insert --platform=${BUILDPLATFORM} into Dockerfile.controller.cross, and preserve the original Dockerfile.controller
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile.controller > Dockerfile.controller.cross
	- $(CONTAINER_TOOL) buildx create --name minecraft-gateway-builder
	$(CONTAINER_TOOL) buildx use minecraft-gateway-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${CONTROLLER_IMG} \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-f Dockerfile.controller.cross .
	- $(CONTAINER_TOOL) buildx rm minecraft-gateway-builder
	rm Dockerfile.controller.cross

##@ Build Edge Envoy
.PHONY: edge-build
edge-build: ## Build edge proxy hostname dynamic module
	cargo build

.PHONY: edge-docker-build
edge-docker-build: ## Build docker image with the edge proxy.
	$(CONTAINER_TOOL) build -t ${EDGE_IMG} . -f Dockerfile.edge

.PHONY: edge-docker-push
edge-docker-push: ## Push docker image with the edge proxy.
	$(CONTAINER_TOOL) push ${EDGE_IMG}

.PHONY: edge-docker-buildx
edge-docker-buildx: ## Build and push docker image for the edge proxy for cross-platform support
	## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile.controller and insert --platform=${BUILDPLATFORM} into Dockerfile.controller.cross, and preserve the original Dockerfile.controller
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile.edge > Dockerfile.edge.cross
	- $(CONTAINER_TOOL) buildx create --name minecraft-edge-builder
	$(CONTAINER_TOOL) buildx use minecraft-edge-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${CONTROLLER_IMG} -f Dockerfile.edge.cross .
	- $(CONTAINER_TOOL) buildx rm minecraft-edge-builder
	rm Dockerfile.edge.cross

##@ Network Integrations

# integration-docker-build builds a docker image for a given integration
# $1 - Integration name
define integration-docker-build
$(CONTAINER_TOOL) build -t ${NETWORK_IMG}-$(1) . -f integrations/$(1)/Dockerfile
endef

# integration-docker-buildx builds and pushes a multi-platform image for a given integration
# $1 - Integration name
define integration-docker-buildx
sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' integrations/$(1)/Dockerfile > integrations/$(1)/Dockerfile.cross
- $(CONTAINER_TOOL) buildx create --name minecraft-$(1)-builder
$(CONTAINER_TOOL) buildx use minecraft-$(1)-builder
- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${NETWORK_IMG}-$(1) -f integrations/$(1)/Dockerfile.cross .
- $(CONTAINER_TOOL) buildx rm minecraft-$(1)-builder
rm integrations/$(1)/Dockerfile.cross
endef

.PHONY: network-docker-build
network-docker-build: proto ## Build all network integration docker images
		$(call foreach-network-integration,integration-docker-build)

.PHONY: network-docker-buildx
network-docker-buildx: proto ## Build and push all network integration docker images for cross-platform support
		$(call foreach-network-integration,integration-docker-buildx)

.PHONY: network-build
network-build: proto ## Build integrations Java library.
	$(call gradlew,build)

.PHONY: network-test
network-test: proto ## Run network integration tests.
	$(call gradlew,:test)

##@ Build
.PHONY: docker-build
docker-build: controller-docker-build edge-docker-build network-docker-build ## Build all docker images

.PHONY: docker-buildx
docker-buildx: controller-docker-buildx edge-docker-buildx network-docker-buildx ## Build and push all docker images with cross-platform support

##@ Push
.PHONY: docker-push
docker-push: controller-docker-push edge-docker-push ## Push all docker images

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply --server-side -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply --server-side -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_IMG}
	cd config/edge && $(KUSTOMIZE) edit set image edge=${EDGE_IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
BUF ?= $(LOCALBIN)/buf

## Tool Versions
KUSTOMIZE_VERSION ?= v5.6.0
CONTROLLER_TOOLS_VERSION ?= v0.18.0
BUF_VERSION ?= v1.50.0
#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')
GOLANGCI_LINT_VERSION ?= v2.8.0

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: buf
buf: $(BUF) ## Download buf locally if necessary.
$(BUF): $(LOCALBIN)
	$(call go-install-tool,$(BUF),github.com/bufbuild/buf/cmd/buf,$(BUF_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef

# gradlew runs a Gradle task in the integrations directory.
# $1 - Gradle task(s) to run (e.g. build, test, :api:test)
define gradlew
cd integrations && ./gradlew $(1)
endef

# foreach-network-integration runs a task for each network integration
# $1 - The Task name
define foreach-network-integration
$(foreach i,$(NETWORK_INTEGRATIONS),$(call $(1),$(i))$(newline))
endef