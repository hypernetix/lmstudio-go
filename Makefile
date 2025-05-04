# Makefile for LM Studio Go

# Variables
PACKAGE := github.com/hypernetix/lmstudio-go
BIN_NAME := lms-go
BUILD_DIR := build
COVERAGE_FILE := coverage/coverage.out
COVERAGE_HTML := coverage/coverage.html

.PHONY: all tests coverage build clean install

# Default target: run tests, measure coverage, and build
all: tests coverage build
tests: unit-tests cli-tests
	@echo "All tests completed"

# Run only unit tests
unit-tests:
	@echo "Running unit tests..."
	go test ./pkg/lmstudio/...

# Run CLI tests
cli-tests:
	@echo "Running CLI tests..."
	go test -tags=integration ./cli

# Measure code coverage
coverage:
	@mkdir -p coverage/raw
	@rm -rf coverage/*.out coverage/*.html coverage/raw/*
	@echo "Generating code coverage for unit tests..."
	go test -covermode=atomic -coverprofile=$(COVERAGE_FILE).unit ./pkg/lmstudio/...
	@echo "Generating code coverage for CLI tests..."
	go test -tags=integration -covermode=atomic -coverprofile=$(COVERAGE_FILE).cli ./cli/
	@echo "Processing instrumented binary coverage data..."
	@if [ -d "coverage/raw" ] && [ "$$(ls -A coverage/raw 2>/dev/null)" ]; then \
		go tool covdata textfmt -i=coverage/raw -o=$(COVERAGE_FILE).instrumented; \
	fi
	@echo "Merging coverage profiles..."
	echo "mode: atomic" > $(COVERAGE_FILE)
	@echo "Processing coverage files..."
	@if [ -f "$(COVERAGE_FILE).instrumented" ]; then \
		grep -h -v "mode: " $(COVERAGE_FILE).unit $(COVERAGE_FILE).cli > $(COVERAGE_FILE).tmp; \
		grep -h -v "mode: " $(COVERAGE_FILE).instrumented | sed "s|$(shell pwd)/|github.com/hypernetix/lmstudio-go/|g" >> $(COVERAGE_FILE).tmp; \
	else \
		grep -h -v "mode: " $(COVERAGE_FILE).unit $(COVERAGE_FILE).cli > $(COVERAGE_FILE).tmp; \
	fi
	@sort -u $(COVERAGE_FILE).tmp >> $(COVERAGE_FILE)
	@rm -f $(COVERAGE_FILE).tmp
	@echo "Coverage summary:"
	@go tool cover -func=$(COVERAGE_FILE)
	@echo "Generating HTML coverage report..."
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report available at: $(COVERAGE_HTML)"
	@echo "Total coverage: $$(go tool cover -func=$(COVERAGE_FILE) | grep total | grep -Eo '[0-9]+\.[0-9]+')%"


# Build the CLI executable
build:
	@echo "Building $(BIN_NAME)..."
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BIN_NAME) cli/lms_go.go

# Install the CLI executable
install:
	@echo "Installing $(BIN_NAME)..."
	go install -v -ldflags "-X main.binaryName=lms-go" ./cli

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
