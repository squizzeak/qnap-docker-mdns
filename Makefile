BINARY   := qnap-docker-mdnsd
CMD_DIR  := ./cmd/qnap-docker-mdnsd
DIST_DIR := ./dist
SHARED   := ./shared

# Default cross-compilation target: x86_64 QNAP
# For ARM QNAP (e.g., TS-xxx), override: make cross-build GOARCH=arm64
# For 32-bit ARM QNAP, override: make cross-build GOARCH=arm
GOOS     ?= linux
GOARCH   ?= amd64

# Container engine: podman preferred, docker fallback
ENGINE   := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
IMAGE    := qnap-docker-mdns-builder
QDK_VER  := 2.5.0

.PHONY: all build cross-build test lint clean run dist help
.PHONY: container-image container-qbuild container-build container-shell

all: build

help:
	@echo "Targets:"
	@echo "  build            - Build for host platform"
	@echo "  cross-build      - Cross-compile for QNAP (GOOS=linux GOARCH=amd64)"
	@echo "  test             - Run all unit tests"
	@echo "  lint             - Run go vet"
	@echo "  clean            - Remove build artifacts"
	@echo "  dist             - Cross-compile and stage binary"
	@echo "  run              - Run daemon locally (requires Docker)"
	@echo ""
	@echo "Container build (uses $(or $(ENGINE),podman/docker)):"
	@echo "  container-image  - Build the QPKG builder image (QDK + Go)"
	@echo "  container-qbuild - Run qbuild inside the container"
	@echo "  container-build  - cross-build + container-qbuild (one step)"
	@echo "  container-shell  - Interactive shell inside the container"
	@echo ""
	@echo "Cross-compilation:"
	@echo "  make cross-build              -> linux/amd64 (x86_64 QNAP)"
	@echo "  make cross-build GOARCH=arm64 -> linux/arm64 (ARM QNAP)"
	@echo "  make cross-build GOARCH=arm   -> linux/arm   (32-bit ARM QNAP)"
	@echo ""
	@echo "For Apple Silicon Macs building for x86_64 QNAP:"
	@echo "  make cross-build GOOS=linux GOARCH=amd64 CGO_ENABLED=0"

build:
	go build -o $(BINARY) $(CMD_DIR)

cross-build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build \
		-o $(DIST_DIR)/$(BINARY)-$(GOARCH) $(CMD_DIR)

dist: cross-build
	@echo "Binary: $(DIST_DIR)/$(BINARY)-$(GOARCH)"
	@echo "Stage with: cp $(DIST_DIR)/$(BINARY)-$(GOARCH) $(SHARED)/$(BINARY)"

test:
	go test -v ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
	rm -rf $(DIST_DIR)/
	rm -f $(SHARED)/$(BINARY)

# Convenience: run locally (requires Docker)
run: build
	./$(BINARY)

# ------------------------------------------------------------------
# Container-based QPKG build (podman preferred, docker fallback)
# ------------------------------------------------------------------

# Build the builder image with QDK + Go toolchain
container-image: check-engine
	$(ENGINE) build \
		--build-arg QDK_VERSION=$(QDK_VER) \
		--build-arg TARGETARCH=$(GOARCH) \
		-t $(IMAGE) \
		-f Dockerfile.build .

# Cross-compile on host, then run qbuild inside the container.
# The repo root is mounted so qbuild writes build/*.qpkg to the
# working tree.
container-build: cross-build container-qbuild

# Run qbuild inside the container.  Expects the binary to already be
# staged in shared/.  Use this when iterating on the QPKG layout
# without recompiling Go.
container-qbuild: check-engine stage-binary
	$(ENGINE) run --rm \
		-v "$(PWD):/build:Z" \
		$(IMAGE) \
		qbuild --force-config
	@echo ""
	@echo "QPKG built:"
	@ls -1 build/*.qpkg 2>/dev/null | xargs -I{} echo "  {}"

# Interactive shell for debugging the build environment
container-shell: check-engine
	$(ENGINE) run --rm -it \
		-v "$(PWD):/build:Z" \
		--entrypoint /bin/bash \
		$(IMAGE)

# ------------------------------------------------------------------
# Helpers
# ------------------------------------------------------------------

stage-binary:
	cp $(DIST_DIR)/$(BINARY)-$(GOARCH) $(SHARED)/$(BINARY)

check-engine:
	@if [ -z "$(ENGINE)" ]; then \
		echo "Error: neither podman nor docker found in PATH."; \
		echo "Install Podman: brew install podman"; \
		echo "Install Docker: https://docker.com"; \
		exit 1; \
	fi
