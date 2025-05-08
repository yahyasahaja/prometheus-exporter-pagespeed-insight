# Default values (can be overridden via environment variables)
GOOS ?= linux
GOARCH ?= amd64
BINARY_NAME ?= psi_exporter
INSTALL_DIR ?= /usr/local/bin

.PHONY: all build build-linux install setup clean

all: build

build:
	@echo "Building for GOOS=$(GOOS), GOARCH=$(GOARCH)"
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINARY_NAME) main.go

build-linux:
	@echo "Building for Linux (GOOS=linux, GOARCH=amd64)"
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME) main.go

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)"
	install -m 0755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)

setup:
	@echo "Initializing Go module and fetching dependencies..."
	go mod init github.com/yourusername/psi-exporter || true
	go get github.com/prometheus/client_golang/prometheus
	go get github.com/prometheus/client_golang/prometheus/promhttp

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
