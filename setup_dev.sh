#!/bin/bash

# Create virtual environment if it doesn't exist
if [ ! -d "venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv venv
fi

# Activate virtual environment
echo "Activating virtual environment..."
source venv/bin/activate

# Install requirements
echo "Installing requirements..."
pip install -r requirements.txt

# Create necessary directories if they don't exist
mkdir -p logs
mkdir -p data
mkdir -p configs

echo "Development environment setup complete!"
echo "To activate the virtual environment, run: source venv/bin/activate"
