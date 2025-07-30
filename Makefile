# Display1306 Makefile

GOTEST ?= go test
BINARY_NAME ?= display1306

.PHONY: build test clean lint fmt help

# Build the display1306 binary
build:
	go build -o $(BINARY_NAME) ./cmd/display1306

# Run tests
test:
	$(GOTEST) ./...

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  build  - Build the display1306 binary"
	@echo "  test   - Run all tests"
	@echo "  clean  - Remove build artifacts"
	@echo "  lint   - Run golangci-lint"
	@echo "  fmt    - Format Go code"
	@echo "  help   - Show this help message"

# Default target
all: fmt lint test build
