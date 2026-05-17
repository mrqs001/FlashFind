BINARY_NAME=flashfind

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOMOD=$(GOCMD) mod
GO_ENV=LC_ALL=en_US.UTF-8 LANG=en_US.UTF-8

DEV_FLAGS=-o $(BINARY_NAME)
RELEASE_FLAGS=-trimpath -ldflags="-s -w" -o $(BINARY_NAME)

.PHONY: all build release clean test vet deps tidy fmt run install help

all: build

build:
	@echo "Building $(BINARY_NAME) (development)..."
	$(GOBUILD) $(DEV_FLAGS)
	@echo "Build complete: $(BINARY_NAME)"

release:
	@echo "Building $(BINARY_NAME) (production)..."
	$(GOBUILD) $(RELEASE_FLAGS)
	@echo "Production build complete: $(BINARY_NAME)"
	@ls -lh $(BINARY_NAME)

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	@echo "Clean complete"

test:
	@echo "Running tests..."
	$(GO_ENV) $(GOTEST) -v ./...

vet:
	@echo "Running go vet..."
	$(GO_ENV) $(GOVET) ./...

deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

fmt:
	@echo "Formatting Go files..."
	gofmt -w *.go

run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install -trimpath -ldflags="-s -w"

help:
	@echo "Available targets:"
	@echo "  build    - Build development version"
	@echo "  release  - Build production version (optimized)"
	@echo "  clean    - Remove build artifacts"
	@echo "  test     - Run tests"
	@echo "  vet      - Run go vet"
	@echo "  deps     - Download dependencies"
	@echo "  tidy     - Tidy go modules"
	@echo "  fmt      - Format Go files"
	@echo "  run      - Build and run the application"
	@echo "  install  - Install to GOPATH/bin"
	@echo "  help     - Show this help message"
