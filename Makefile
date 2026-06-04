BINARY   := qnap-docker-mdnsd
CMD_DIR  := ./cmd/qnap-docker-mdnsd
DIST_DIR := ./dist

GOOS     ?= linux
GOARCH   ?= amd64

.PHONY: all build cross-build test lint clean

all: build

build:
	go build -o $(BINARY) $(CMD_DIR)

cross-build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build \
		-o $(DIST_DIR)/$(BINARY)-$(GOARCH) $(CMD_DIR)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
	rm -rf $(DIST_DIR)/
