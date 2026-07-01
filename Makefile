# Makefile for Agent Flow Operator
# Uses agent-sandbox for isolated worker execution

IMAGE_REGISTRY ?= ghcr.io
IMAGE_REPO ?= minagflow/agent-flow-planner
IMAGE_TAG ?= latest
IMAGE_NAME = $(IMAGE_REGISTRY)/$(IMAGE_REPO)/agent-flow-planner:$(IMAGE_TAG)

WEB_IMAGE_NAME = $(IMAGE_REGISTRY)/minagflow/agent-flow-web:$(IMAGE_TAG)

# Tools
CONTROLLER_GEN ?= $(GOBIN)/controller-gen
ENVTEST ?= $(GOBIN)/setup-envtest
GOLINTER ?= $(GOBIN)/golangci-lint

# Go binary directory
GOBIN ?= $(shell go env GOPATH)/bin

# Settings
KUBECONFIG ?= $(HOME)/.kube/config
NAMESPACE ?= agent-flow-system

.PHONY: all build run deploy undeploy test generate manifests docker-build docker-push clean install-agent-sandbox uninstall-agent-sandbox controller-gen kustomize deploy-planner-cluster

all: build

# Build the controller binary
build: generate fmt vet
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/planner cmd/planner/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/planner/main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE_BUILD) config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE_BUILD) config/crd | kubectl delete -f -

# Deploy planner controller to cluster (build image + rollout manager)
deploy-planner-cluster:
	./scripts/deploy-planner-cluster.sh --local

# Deploy controller to a cluster
deploy: manifests kustomize
ifneq ($(KUSTOMIZE_BIN),)
	cd config/manager && $(KUSTOMIZE_BIN) edit set image controller=$(IMAGE_NAME)
endif
	$(KUSTOMIZE_BUILD) config/default | kubectl apply -f -

# Undeploy controller from a cluster
undeploy: manifests kustomize
	$(KUSTOMIZE_BUILD) config/default | kubectl delete -f -

# Run tests
test: envtest
	source $(ENVTEST_BIN)/test; go test ./... -v -coverprofile cover.out

# Generate manifests (CRD + RBAC)
manifests: controller-gen
	$(CONTROLLER_GEN) crd rbac:roleName=manager-role paths="./..." output:crd:artifacts:config=config/crd/bases

# Generate deepcopy code
generate:
	$(CONTROLLER_GEN) object paths="./api/..."

controller-gen:
	@echo "Using controller-gen from $(CONTROLLER_GEN)"

# Format the code
fmt:
	go fmt ./...

# Vet the code
vet:
	go vet ./...

# Run linter
lint: golinter
	$(GOLINTER) run ./...

# Download dependencies
deps:
	go mod download

# Build Docker image
MCP_IMAGE_NAME = $(IMAGE_REGISTRY)/minagflow/mcp-sidecar:$(IMAGE_TAG)
WORKER_IMAGE_NAME = $(IMAGE_REGISTRY)/minagflow/worker-agent:$(IMAGE_TAG)

docker-build:
	docker build -t $(IMAGE_NAME) -f Dockerfile .

# Build MCP sidecar Docker image
docker-build-mcp:
	docker build -t $(MCP_IMAGE_NAME) -f Dockerfile.mcp .

# Build worker agent Docker image
docker-build-worker:
	docker build -t $(WORKER_IMAGE_NAME) -f Dockerfile.worker .

# Build web Docker image
docker-build-web:
	docker build -t $(WEB_IMAGE_NAME) -f Dockerfile.web .

# Push Docker image
docker-push:
	docker push $(IMAGE_NAME)

# Push MCP sidecar Docker image
docker-push-mcp:
	docker push $(MCP_IMAGE_NAME)

# Push worker agent Docker image
docker-push-worker:
	docker push $(WORKER_IMAGE_NAME)

# Push web Docker image
docker-push-web:
	docker push $(WEB_IMAGE_NAME)

# Build all images
docker-build-all: docker-build docker-build-mcp docker-build-worker docker-build-web

# Deploy web to cluster
deploy-web:
	$(KUSTOMIZE) build config/web | kubectl apply -f -

.PHONY: web-open
web-open:
	./run-web.sh

# Undeploy web from cluster
undeploy-web:
	$(KUSTOMIZE) build config/web | kubectl delete -f -

# Setup envtest for testing
setup-envtest: $(ENVTEST)

$(CONTROLLER_GEN):
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

$(ENVTEST):
	cd /tmp && go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

$(GOLINTER):
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Kustomize (falls back to built-in kubectl kustomize when binary is absent)
KUSTOMIZE_BIN := $(shell command -v kustomize 2>/dev/null)
ifeq ($(KUSTOMIZE_BIN),)
KUSTOMIZE_BUILD = kubectl kustomize
else
KUSTOMIZE_BUILD = $(KUSTOMIZE_BIN) build
endif
kustomize:
	@echo "Using kustomize: $(if $(KUSTOMIZE_BIN),$(KUSTOMIZE_BIN),kubectl kustomize)"
.PHONY: kustomize

# Clean build artifacts
clean:
	rm -rf bin/

# Full rebuild
rebuild: clean all

# Deploy official agent-sandbox controller (required execution substrate for Sandbox CRs)
# Uses the vendored manifest (v0.5.0) for reproducibility. This deploys the controller
# in agent-sandbox-system namespace and installs the agents.x-k8s.io CRDs.
install-agent-sandbox:
	kubectl apply -f config/agent-sandbox/manifest.yaml

# Uninstall agent-sandbox controller and CRDs
uninstall-agent-sandbox:
	kubectl delete -f config/agent-sandbox/manifest.yaml --ignore-not-found || true

# Convenience: deploy everything (your operator + agent-sandbox substrate)
deploy-all: install-agent-sandbox deploy

# Convenience: full cleanup
undeploy-all: undeploy uninstall uninstall-agent-sandbox
