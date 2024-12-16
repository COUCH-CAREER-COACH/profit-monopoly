.PHONY: all build test bench clean lint coverage

# Build settings
BINARY_NAME=mev-bot
GO=go
GOFLAGS=-v
BUILD_DIR=build
COVERAGE_DIR=coverage

all: clean lint test build

build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/...

test:
	@echo "Running tests..."
	$(GO) test ./... -v -race

bench:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./...

coverage:
	@echo "Generating coverage report..."
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test ./... -coverprofile=$(COVERAGE_DIR)/coverage.out
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html

lint:
	@echo "Running linters..."
	golangci-lint run ./...

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf $(COVERAGE_DIR)
	$(GO) clean -testcache

# Development helpers
dev: lint test build
	@echo "Development build complete"

# CI targets
ci: lint test coverage
	@echo "CI pipeline complete"
