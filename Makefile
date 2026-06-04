BINARY   := qnap-docker-mdnsd
CMD_DIR  := ./cmd/qnap-docker-mdnsd
DIST_DIR := ./dist
SHARED   := ./shared

GOOS     ?= linux
GOARCH   ?= amd64

# Container engine: podman preferred, docker fallback.
# Use '=' so the shell command is evaluated on first use (not at parse
# time), which keeps 'make help' / 'make build' fast when no engine is
# installed.
ENGINE   = $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
IMAGE    := qnap-docker-mdns-builder
QDK_VER  := 2.5.0

.PHONY: all build cross-build test lint clean run dist help
.PHONY: container-image container-qbuild container-build container-sign container-shell install release
.PHONY: check-engine

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
	@if [ -n "$$(command -v podman 2>/dev/null || command -v docker 2>/dev/null)" ]; then \
		echo "Container build (uses $$(command -v podman 2>/dev/null || command -v docker 2>/dev/null)):"; \
		echo "  container-image  - Build the QPKG builder image (QDK + Go)"; \
		echo "  container-qbuild - Run qbuild inside the container"; \
		echo "  container-build  - cross-build + container-qbuild (one step)"; \
		echo "  container-sign   - container-build + add code signing signature"; \
		echo "  container-shell  - Interactive shell inside the container"; \
		echo "  install          - Build, upload, install & enable on NAS via SSH"; \
		echo "  release          - Tag and push a release (make release VER=v1.0.0)"; \
	else \
		echo "Container build: install podman or docker to enable"; \
		echo "  container-image, container-qbuild, container-build,"; \
		echo "  container-sign, container-shell, install, release"; \
	fi
	@echo ""
	@echo "Code signing requires QNAP_CODESIGNING_TOKEN env variable."
	@echo "Get it from https://www.qnap.com/ (Developer Partner portal)."
	@echo ""
	@echo "Cross-compilation:"
	@echo "  make cross-build              -> linux/amd64 (x86_64 QNAP)"
	@echo "  make cross-build GOARCH=arm64 -> linux/arm64 (ARM QNAP)"
	@echo "  make cross-build GOARCH=arm   -> linux/arm   (32-bit ARM QNAP)"
	@echo ""
	@echo "For Apple Silicon Macs building for x86_64 QNAP:"
	@echo "  make cross-build GOOS=linux GOARCH=amd64 CGO_ENABLED=0"
	@echo "  # Container build requires QEMU binfmt_misc (Podman:"
	@echo "  # podman machine ssh -- sudo podman run --rm --arch amd64 alpine true)"

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

run: build
	./$(BINARY)

# ------------------------------------------------------------------
# Container-based QPKG build (podman preferred, docker fallback)
# ------------------------------------------------------------------

container-image: check-engine
	$(ENGINE) build \
		--platform linux/amd64 \
		--build-arg QDK_VERSION=$(QDK_VER) \
		--build-arg TARGETARCH=$(GOARCH) \
		-t $(IMAGE) \
		-f Dockerfile.build .

container-build: cross-build container-qbuild

container-qbuild: check-engine stage-binary
	$(ENGINE) run --rm \
		-v "$(PWD):/build:Z" \
		-e QDK_PACKAGE_ROUTINES=/usr/share/QDK/template/package_routines \
		$(IMAGE) \
		qbuild --force-config
	@echo ""
	@echo "QPKG built:"
	@ls -1 build/*.qpkg 2>/dev/null | xargs -I{} echo "  {}"

# Build and add QNAP code signing.  Requires QNAP_CODESIGNING_TOKEN in
# the environment (or as a GitHub Actions secret).  The token is
# obtained from QNAP's Developer Partner portal.
container-sign: container-qbuild
	@if [ -z "$$QNAP_CODESIGNING_TOKEN" ]; then \
		echo "Error: QNAP_CODESIGNING_TOKEN not set."; \
		echo "Get it from https://www.qnap.com/ (Developer Partner portal)."; \
		echo "Set it via: export QNAP_CODESIGNING_TOKEN=..."; \
		exit 1; \
	fi
	$(ENGINE) run --rm \
		-v "$(PWD):/build:Z" \
		-e QNAP_CODESIGNING_TOKEN \
		$(IMAGE) \
		qbuild --add-code-signing "$$(ls build/*.qpkg | head -1)"
	@echo ""
	@echo "Signed QPKG:"
	@ls -1 build/*.qpkg 2>/dev/null | xargs -I{} echo "  {}"

# Upload and install the QPKG on a QNAP NAS via SSH.
# Requires the App Center setting "Allow installation of applications
# without a valid digital signature" to be enabled on the NAS.
# Override with: make install NAS_HOST=my-nas.local
NAS_HOST ?= qnap.local

install: container-build
	scp build/qnap-docker-mdns_1.0.0.qpkg admin@$(NAS_HOST):/tmp/qnap-docker-mdns.qpkg
	ssh admin@$(NAS_HOST) "qpkg_cli -m /tmp/qnap-docker-mdns.qpkg -A && qpkg_cli --enable qnap-docker-mdns"
	@echo ""
	@echo "Installed and enabled on $(NAS_HOST)."
	@echo "Check status: ssh admin@$(NAS_HOST) qpkg_cli -s qnap-docker-mdns --output 2"

# Tag and push a release (triggers the GitHub Actions workflow).
# Usage: make release VER=v1.0.0
release:
	@if [ -z "$(VER)" ]; then \
		echo "Usage: make release VER=v1.0.0"; \
		exit 1; \
	fi
	git tag $(VER)
	git push origin $(VER)
	@echo ""
	@echo "Tag $(VER) pushed.  GitHub Actions will build and publish the release:"
	@echo "  https://github.com/squizzeak/qnap-docker-mdns/actions"

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

# check-engine finds a working container runtime at recipe time (so
# targets like 'make help' / 'make build' never trigger the check).
check-engine:
	@ENGINE=$$(command -v podman 2>/dev/null || command -v docker 2>/dev/null); \
	if [ -z "$$ENGINE" ]; then \
		echo "Error: neither podman nor docker found in PATH."; \
		echo "Install Podman: brew install podman"; \
		echo "Install Docker: https://docker.com"; \
		exit 1; \
	fi
