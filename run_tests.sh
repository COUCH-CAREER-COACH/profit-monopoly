#!/bin/bash

# Activate virtual environment if it exists
if [ -d "venv" ]; then
    source venv/bin/activate
fi

# Install test dependencies if not already installed
pip install pytest pytest-asyncio pytest-cov

# Run tests with coverage
pytest tests/ -v --cov=mevbot --cov-report=term-missing
