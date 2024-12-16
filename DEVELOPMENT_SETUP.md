# Development Setup Guide

## Virtual Environment Setup

1. Create a new virtual environment:
```bash
python3 -m venv venv
```

2. Activate the virtual environment:
- On macOS/Linux:
```bash
source venv/bin/activate
```
- On Windows:
```bash
.\venv\Scripts\activate
```

3. Install required packages:
```bash
pip install -r requirements.txt
```

## Environment Variables

1. Copy the template environment file:
```bash
cp mev-templates/python/.env.example mev-templates/python/.env
```

2. Update the following variables in `.env`:
- `ETHEREUM_SEPOLIA_RPC_URL`: Your Sepolia testnet RPC URL
- `WSS_URL_SEPOLIA`: WebSocket endpoint for Sepolia
- `PRIVATE_KEY`: Your Ethereum private key (without 0x prefix)
- `FLASHBOTS_RELAY_URL`: Flashbots relay URL

## Running Tests

1. Ensure your virtual environment is activated:
```bash
source venv/bin/activate  # macOS/Linux
```

2. Run all tests:
```bash
python -m pytest tests/
```

3. Run specific test files:
```bash
python -m pytest tests/test_integration.py -v  # Verbose mode
```

4. Run tests with specific markers:
```bash
python -m pytest -v -m "not slow"  # Skip slow tests
```

## Common Issues

1. **Permission Denied on venv/bin/activate**
   ```bash
   chmod +x venv/bin/activate
   ```

2. **Missing Dependencies**
   ```bash
   pip install -r requirements.txt
   ```

3. **Test Failures**
   - Ensure all environment variables are set correctly
   - Check that you're using the latest dependencies
   - Verify your Ethereum node connection

## Development Best Practices

1. **Before Running Tests**
   - Activate virtual environment
   - Verify environment variables
   - Ensure test network (Sepolia) is accessible

2. **After Code Changes**
   - Run unit tests first
   - Run integration tests
   - Check test coverage

3. **Environment Management**
   - Keep virtual environment isolated
   - Update requirements.txt when adding new dependencies
   - Document any new environment variables

## MEV-Specific Requirements

1. **System Requirements**
   - CPU: Minimum 8 cores for parallel transaction processing
   - Memory: Minimum 16GB RAM for mempool monitoring
   - Storage: NVMe SSD with at least 500GB free space
   - Network: Low-latency connection (< 50ms to Ethereum nodes)

2. **Network Optimization**
   ```bash
   # Increase max file descriptors for network connections
   ulimit -n 65535
   
   # Enable TCP BBR congestion control (Linux only)
   sudo modprobe tcp_bbr
   echo "tcp_bbr" | sudo tee /etc/modules-load.d/bbr.conf
   ```

3. **Memory Optimization**
   ```bash
   # Enable huge pages (Linux only)
   sudo sysctl -w vm.nr_hugepages=1024
   
   # Disable swap for consistent performance
   sudo swapoff -a
   ```

4. **Node Requirements**
   - Dedicated Ethereum node (Geth/Erigon)
   - Multiple RPC endpoints for redundancy
   - WebSocket connection for real-time updates
   - Archive node access for historical data

## Performance Testing

1. **Latency Testing**
```bash
# Test node latency
python -m pytest tests/performance/test_latency.py

# Test transaction submission time
python -m pytest tests/performance/test_tx_submission.py
```

2. **Memory Testing**
```bash
# Test mempool monitoring memory usage
python -m pytest tests/performance/test_mempool_memory.py

# Test flash loan optimization memory
python -m pytest tests/performance/test_flashloan_memory.py
```

3. **Load Testing**
```bash
# Test parallel transaction processing
python -m pytest tests/performance/test_parallel_tx.py

# Test multiple DEX monitoring
python -m pytest tests/performance/test_dex_monitoring.py
```

## Security Setup

1. **Key Management**
   - Use environment variables for all sensitive data
   - Never commit private keys or API keys
   - Rotate keys regularly
   - Use separate keys for testing and production

2. **Network Security**
   - Use VPN for node connections
   - Implement rate limiting
   - Monitor for suspicious patterns
   - Set up alerts for unusual activity

3. **Testing Security**
   ```bash
   # Run security checks
   python -m pytest tests/security/test_key_protection.py
   python -m pytest tests/security/test_input_validation.py
   ```

## Monitoring Setup

1. **Logging Configuration**
   ```python
   # In your .env file
   LOG_LEVEL=DEBUG
   LOG_FORMAT=json
   LOG_FILE=/var/log/mevbot/mevbot.log
   ```

2. **Metrics Collection**
   ```bash
   # Start Prometheus metrics server
   python scripts/start_metrics.py
   
   # Test metrics collection
   python -m pytest tests/monitoring/test_metrics.py
   ```

3. **Alert Setup**
   ```bash
   # Configure alert thresholds
   python scripts/configure_alerts.py
   
   # Test alert system
   python -m pytest tests/monitoring/test_alerts.py
   ```

## Development Workflow

1. **Before Starting**
   ```bash
   # Update dependencies
   pip install -r requirements.txt
   
   # Run database migrations
   python scripts/migrate.py
   
   # Start required services
   python scripts/start_services.py
   ```

2. **During Development**
   ```bash
   # Run linting
   flake8 mevbot tests
   
   # Run type checking
   mypy mevbot
   
   # Run unit tests
   python -m pytest tests/unit
   ```

3. **Before Committing**
   ```bash
   # Run full test suite
   python -m pytest
   
   # Check test coverage
   python -m pytest --cov=mevbot
   
   # Run security checks
   python scripts/security_check.py
   ```
