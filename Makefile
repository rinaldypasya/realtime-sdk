.PHONY: all build test test-unit test-integration lint fmt clean deps run-example help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Binary
BINARY_NAME=realtime-sdk
EXAMPLE_BINARY=example

# Directories
PKG_DIR=./pkg/...
INTERNAL_DIR=./internal/...
CMD_DIR=./cmd/...
TEST_UNIT_DIR=./test/unit/...
TEST_INT_DIR=./test/integration/...

all: deps lint test build

## Build
build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(CMD_DIR)

build-example:
	$(GOBUILD) -o $(EXAMPLE_BINARY) -v ./cmd/example/

## Test
test: test-unit test-integration

test-unit:
	$(GOTEST) -v -race $(TEST_UNIT_DIR)

test-integration:
	$(GOTEST) -v -race $(TEST_INT_DIR)

test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out $(PKG_DIR) $(INTERNAL_DIR) $(TEST_UNIT_DIR)
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-verbose:
	$(GOTEST) -v -race -count=1 $(PKG_DIR) $(INTERNAL_DIR) $(TEST_UNIT_DIR) $(TEST_INT_DIR)

## Lint and Format
lint:
	$(GOLINT) run $(PKG_DIR) $(INTERNAL_DIR) $(CMD_DIR) || true

fmt:
	$(GOFMT) -s -w .

fmt-check:
	@test -z "$$($(GOFMT) -l .)" || (echo "Please run 'make fmt'" && exit 1)

vet:
	$(GOCMD) vet $(PKG_DIR) $(INTERNAL_DIR) $(CMD_DIR)

## Dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

deps-update:
	$(GOMOD) tidy
	$(GOGET) -u $(PKG_DIR)

## Clean
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(EXAMPLE_BINARY)
	rm -f coverage.out
	rm -f coverage.html

## Run
run-example: build-example
	./$(EXAMPLE_BINARY)

## Documentation
docs:
	@echo "Generating documentation..."
	godoc -http=:6060 &
	@echo "Documentation server running at http://localhost:6060"

## Help
help:
	@echo "Realtime SDK - Makefile Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make <command>"
	@echo ""
	@echo "Commands:"
	@echo "  all              Run deps, lint, test, and build"
	@echo "  build            Build the SDK"
	@echo "  build-example    Build the example application"
	@echo "  test             Run all tests"
	@echo "  test-unit        Run unit tests"
	@echo "  test-integration Run integration tests"
	@echo "  test-coverage    Run tests with coverage report"
	@echo "  lint             Run linter"
	@echo "  fmt              Format code"
	@echo "  fmt-check        Check code formatting"
	@echo "  vet              Run go vet"
	@echo "  deps             Download dependencies"
	@echo "  deps-update      Update dependencies"
	@echo "  clean            Clean build artifacts"
	@echo "  run-example      Build and run the example"
	@echo "  docs             Generate documentation"
	@echo "  help             Show this help message"
