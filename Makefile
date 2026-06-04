BINARY   := qnap-docker-mdnsd
CMD_DIR  := ./cmd/qnap-docker-mdnsd
DIST_DIR := ./dist
SHARED   := ./shared
QPKG_DIR := ./qpkg

# Default cross-compilation target: x86_64 QNAP
# For ARM QNAP (e.g., TS-xxx), override: make cross-build GOARCH=arm64
# For 32-bit ARM QNAP, override: make cross-build GOARCH=arm
GOOS     ?= linux
GOARCH   ?= amd64

.PHONY: all build cross-build test lint clean run dist qpkg help

all: build

help:
	@echo "Targets:"
	@echo "  build        - Build for host platform"
	@echo "  cross-build  - Cross-compile for QNAP (GOOS=linux GOARCH=amd64)"
	@echo "  test         - Run all unit tests"
	@echo "  lint         - Run go vet"
	@echo "  clean        - Remove build artifacts"
	@echo ""
	@echo "Cross-compilation defaults:"
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
	@echo "Copy to shared/ for QPKG inclusion:"
	@echo "  cp $(DIST_DIR)/$(BINARY)-$(GOARCH) $(SHARED)/$(BINARY)"

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

# QPKG assembly (requires qbuild from QDK)
# Unpacked iteration: install qpkg manually, modify qinstall.sh for speed
qpkg: cross-build
	cp $(DIST_DIR)/$(BINARY)-$(GOARCH) $(SHARED)/$(BINARY)
	@echo "Now run: qbuild"
	@echo "Or for unpacked iteration:"
	@echo "  # Extract built qpkg, edit shared/, re-run qinstall.sh"
