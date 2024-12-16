#!/bin/bash

# Exit on error
set -e

echo "Setting up development environment..."

# Check Go installation
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

# Install development tools
echo "Installing development tools..."
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/golang/mock/mockgen@latest

# Create necessary directories
mkdir -p build coverage logs

# Check if .env exists, if not copy from example
if [ ! -f .env ]; then
    if [ -f .env.example ]; then
        cp .env.example .env
        echo "Created .env from .env.example - please update with your configuration"
    fi
fi

# Install dependencies
echo "Installing project dependencies..."
go mod download
go mod tidy

# Run linter
echo "Running linter..."
if command -v golangci-lint &> /dev/null; then
    golangci-lint run
else
    echo "Warning: golangci-lint not found in PATH"
fi

# Run tests
echo "Running tests..."
go test ./... -v

echo "Development environment setup complete!"
echo "Next steps:"
echo "1. Update your .env file with your configuration"
echo "2. Review config.example.json and create your config.json"
echo "3. Run 'make build' to build the project"
