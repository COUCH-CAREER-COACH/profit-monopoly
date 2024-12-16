#!/bin/bash

set -e

echo "Building Docker image..."
docker build -t mevbot-test -f Dockerfile ..

echo "Running tests..."
docker run --rm mevbot-test
