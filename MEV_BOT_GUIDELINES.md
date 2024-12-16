# MEV Bot Development Guidelines

## 1. Project Structure
```
mev-bot/
├── contracts/               # Solidity smart contracts
│   ├── src/                # Contract source files
│   ├── script/             # Deployment scripts
│   └── test/              # Contract tests
├── python/                 # Python bot implementation
│   ├── src/               # Source code
│   │   ├── core/         # Core bot functionality
│   │   ├── strategies/   # Trading strategies
│   │   ├── utils/        # Utility functions
│   │   └── config/       # Configuration files
│   ├── tests/            # Python tests
│   └── scripts/          # Utility scripts
└── docs/                  # Documentation
```

## 2. Code Organization Rules

### 2.1 Smart Contracts
- One contract per file
- Clear inheritance hierarchy
- Comprehensive NatSpec documentation
- Gas optimization patterns
- Security-first approach

### 2.2 Python Code
- Type hints mandatory
- Async/await for all I/O operations
- Comprehensive logging
- Error handling with custom exceptions
- Configuration via environment variables

## 3. Development Workflow

### 3.1 Version Control
- Feature branches for new development
- Commit messages follow conventional commits
- No secrets in version control
- Regular commits with atomic changes

### 3.2 Testing
- Unit tests for all new code
- Integration tests for strategies
- Local network testing before mainnet
- Gas optimization tests

## 4. Security Guidelines

### 4.1 Key Management
- Never commit private keys
- Use .env files for local development
- Separate keys for testing and production
- Regular key rotation

### 4.2 Contract Security
- Access control for all functions
- Circuit breakers for emergencies
- Rate limiting where appropriate
- Reentrancy protection

### 4.3 Network Security
- Dedicated nodes for production
- Redundant node connections
- WebSocket for real-time data
- Fallback RPC providers

## 5. Performance Guidelines

### 5.1 Bot Performance
- Memory-mapped files for speed
- Custom mempool indexing
- Parallel transaction processing
- CPU pinning for critical threads

### 5.2 Gas Optimization
- Dynamic gas pricing
- Bundle optimization
- Transaction batching
- Failed transaction analysis

## 6. Monitoring and Maintenance

### 6.1 Logging
- Structured logging format
- Different log levels (DEBUG, INFO, WARNING, ERROR)
- Rotation and archival
- Performance metrics

### 6.2 Alerting
- Profit/loss monitoring
- System health checks
- Gas price alerts
- Error rate monitoring

## 7. Strategy Development

### 7.1 Strategy Rules
- Modular strategy design
- Clear entry/exit conditions
- Risk management parameters
- Profit calculation logic

### 7.2 Backtest Requirements
- Historical data validation
- Gas cost simulation
- Slippage modeling
- Competition analysis

## 8. Production Deployment

### 8.1 Infrastructure
- High-availability setup
- Geographic distribution
- Automated failover
- Regular backups

### 8.2 Deployment Process
- Staged rollout (testnet → mainnet)
- Version control tags
- Deployment documentation
- Rollback procedures

## 9. Development Setup

For detailed instructions on setting up the development environment, running tests, and handling common issues, please refer to [DEVELOPMENT_SETUP.md](DEVELOPMENT_SETUP.md).

Key points:
1. Always use a virtual environment
2. Keep environment variables secure and up-to-date
3. Run tests before and after making changes
4. Follow the test execution guidelines
