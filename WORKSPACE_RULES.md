# Workspace Rules for Windsurf MEV Bot

## High-Performance Architecture Requirements

### 1. Ultra-Low Latency Components
- Memory-mapped transaction processing
- Lock-free data structures for mempool
- DPDK network stack integration
- Kernel bypass for network packets
- CPU pinning for critical threads
- Huge pages for memory allocation
- Custom memory allocator for hot paths
- Zero-copy network processing

### 2. Advanced MEV Infrastructure
- Custom mempool indexer with SIMD
- P2P network optimization
- Private transaction channels
- Builder relationships management
- Cross-domain MEV detection
- Multi-chain support architecture
- Probabilistic transaction ordering
- Real-time price impact calculation

### 3. Competitive Edge Components
- Custom P2P network with builder connections
- Private mempool relationships
- Specialized block builder partnerships
- Priority transaction channels
- Cross-domain MEV correlation engine
- ML-based opportunity prediction
- Hardware-accelerated signature verification
- FPGA-based transaction filtering

### 4. Advanced Analytics
- Real-time profitability surface mapping
- Competitor behavior modeling
- Market impact prediction engine
- Protocol state prediction
- Gas market modeling
- Block builder behavior analysis
- Network topology optimization
- Liquidity flow tracking

## Project Structure
- `/cmd` - Command-line interfaces and entry points
- `/config` - Configuration management
- `/mempool` - Mempool monitoring and indexing
- `/strategies` - MEV strategy implementations
- `/flashloan` - Flash loan integrations
- `/bundler` - Flashbots bundle management
- `/utils` - Shared utilities and helpers
- `/network` - Custom network stack
- `/metrics` - Performance tracking
- `/simulation` - Strategy testing
- `/backtest` - Historical analysis

## Development Framework

### 1. Performance Requirements
1. **Transaction Processing**
   - New block processing < 5ms
   - Transaction validation < 100μs
   - Bundle creation < 1ms
   - Network latency < 10ms
   - Memory allocation < 1μs
   - Zero GC during critical paths

2. **System Resources**
   - CPU usage < 60% per core
   - Memory usage < 4GB
   - Network buffer < 100MB
   - Zero system calls in hot path
   - Cache miss rate < 1%
   - Page faults < 1/sec

3. **MEV Performance**
   - Bundle success rate > 80%
   - Profit calculation < 50μs
   - Gas estimation error < 2%
   - Slippage prediction accuracy > 98%
   - Competition detection < 100μs
   - Risk assessment < 10μs

### 2. Feature Implementation Lifecycle
1. **Planning Phase**
   - Performance requirements definition
   - Latency budget allocation
   - Resource utilization plans
   - Bottleneck identification
   - Competition analysis
   - Risk assessment matrix

2. **Implementation Phase**
   - Start with performance benchmarks
   - Implement core algorithms
   - Optimize critical paths
   - Profile and tune
   - Security hardening
   - Metrics integration

3. **Testing Phase**
   - Latency testing (99.9th percentile)
   - Load testing under market stress
   - Network congestion simulation
   - Competition simulation
   - Profit/loss scenarios
   - Security penetration testing

### 3. MEV Strategy Requirements
1. **Strategy Optimization**
   - Real-time profitability calculation
   - Dynamic gas price adjustment
   - Multi-path execution planning
   - Sandwich attack detection
   - Frontrunning protection
   - Backrunning optimization
   - Cross-protocol arbitrage
   - Liquidation opportunity detection

2. **Risk Management**
   - Real-time position monitoring
   - Dynamic exposure limits
   - Automatic circuit breakers
   - Slippage protection
   - Gas price protection
   - Competition monitoring
   - Network health checks
   - Protocol risk assessment

3. **Performance Optimization**
   - SIMD instruction usage
   - Custom memory management
   - Lock-free algorithms
   - Branch prediction optimization
   - Cache-friendly data structures
   - Parallel execution paths
   - Network stack optimization
   - Zero-copy processing

4. **Advanced Strategy Components**
   - Cross-domain arbitrage detection
   - Multi-block MEV extraction
   - Sandwich opportunity scoring
   - Liquidation prediction engine
   - NFT MEV opportunity detection
   - DEX inefficiency exploitation
   - Long-tail token opportunities
   - Protocol upgrade MEV detection

### 4. Testing Standards
1. **Performance Testing**
   - Latency profiling
   - Memory allocation tracking
   - Cache miss analysis
   - Network packet analysis
   - System call profiling
   - Lock contention measurement
   - GC impact analysis
   - CPU utilization patterns

2. **Strategy Testing**
   - Historical backtest analysis
   - Monte Carlo simulations
   - Competition modeling
   - Market impact simulation
   - Network congestion tests
   - Protocol edge cases
   - Failure scenario testing
   - Recovery testing

### 5. Monitoring Requirements
1. **Performance Metrics**
   - Transaction latency histogram
   - Memory allocation rates
   - Cache hit/miss rates
   - Network packet latency
   - System call frequency
   - Lock contention rates
   - GC pause times
   - CPU core utilization

2. **Business Metrics**
   - Profit per opportunity
   - Success rate by strategy
   - Gas costs analysis
   - Competition win rate
   - Market impact measurement
   - Risk exposure levels
   - Protocol utilization
   - Capital efficiency

### 6. Security Requirements
1. **Transaction Security**
   - Private key protection
   - Transaction signing isolation
   - RPC endpoint security
   - Network packet encryption
   - Memory protection
   - Access control
   - Audit logging
   - Intrusion detection

2. **Operation Security**
   - Deployment security
   - Configuration protection
   - Network isolation
   - Access logging
   - Change management
   - Incident response
   - Recovery procedures
   - Security audits

### 7. Emergency Procedures
1. **Circuit Breakers**
   - Loss limits
   - Exposure limits
   - Gas price limits
   - Network health
   - Protocol risk
   - Competition pressure
   - System resource limits
   - Error rate thresholds

2. **Recovery Procedures**
   - Emergency shutdown
   - Position unwinding
   - Fund recovery
   - System restore
   - Network failover
   - State recovery
   - Incident analysis
   - Stakeholder communication

### 8. Infrastructure Requirements
1. **Network Infrastructure**
   - Dedicated fiber connections
   - Multiple datacenter presence
   - Direct builder connections
   - Custom P2P overlay network
   - Priority transaction channels
   - Redundant connectivity
   - Low-latency DNS
   - Network route optimization

2. **Hardware Optimization**
   - FPGA acceleration
   - Custom NIC firmware
   - Kernel bypass networking
   - CPU core isolation
   - NUMA optimization
   - Cache optimization
   - Memory interleaving
   - DMA optimization

3. **Software Architecture**
   - Lock-free algorithms
   - Custom memory allocators
   - Vectorized processing
   - JIT compilation
   - Hot path optimization
   - Branch prediction tuning
   - Cache line alignment
   - Assembly optimization

### 9. Competitive Analysis
1. **Market Intelligence**
   - Builder relationship mapping
   - Competitor strategy analysis
   - Protocol upgrade tracking
   - Gas market analysis
   - Network topology mapping
   - Block propagation analysis
   - MEV extraction patterns
   - Liquidity flow analysis

2. **Strategy Adaptation**
   - Dynamic strategy switching
   - Real-time profit thresholds
   - Competitive pressure detection
   - Market impact adaptation
   - Gas price optimization
   - Bundle timing optimization
   - Protocol state adaptation
   - Risk exposure management

## Quality Gates
1. **Performance Gates**
   - Latency requirements met
   - Resource usage within limits
   - Zero GC in critical path
   - Network optimization verified
   - Memory allocation patterns
   - Cache efficiency verified
   - System call patterns
   - Lock contention measured

2. **Strategy Gates**
   - Profit calculation verified
   - Risk controls tested
   - Competition analysis complete
   - Market impact measured
   - Gas optimization verified
   - Protocol integration tested
   - Recovery scenarios verified
   - Monitoring configured

3. **Security Gates**
   - Security audit complete
   - Penetration testing done
   - Access controls verified
   - Encryption verified
   - Audit logging confirmed
   - Recovery tested
   - Incident response ready
   - Configuration secured

## Deployment Rules
1. **Pre-deployment**
   - All performance tests passed
   - Security verification complete
   - Strategy simulation successful
   - Monitoring configured
   - Alerts tested
   - Backup systems ready
   - Recovery tested
   - Documentation complete

2. **Deployment Process**
   - Gradual capital allocation
   - Strategy activation sequence
   - Performance monitoring
   - Risk monitoring
   - Competition monitoring
   - Protocol monitoring
   - System monitoring
   - Incident readiness

## Version Control
- Performance impact documented
- Benchmark results included
- Security review documented
- Risk assessment included
- Strategy changes documented
- Configuration changes noted
- Monitoring updates included
- Recovery procedures updated

## Advanced Development Practices
1. **Code Optimization**
   - Assembly-level optimization
   - SIMD instruction sets
   - Cache-aware algorithms
   - Branch prediction hints
   - Prefetch optimization
   - Function inlining
   - Hot/cold path separation
   - Critical path optimization

2. **Performance Analysis**
   - Cycle-accurate profiling
   - Cache miss analysis
   - Branch prediction stats
   - Memory access patterns
   - System call overhead
   - Network packet analysis
   - Interrupt handling
   - Context switch monitoring

3. **System Tuning**
   - OS scheduler optimization
   - Network stack tuning
   - Memory management tuning
   - I/O scheduler optimization
   - IRQ affinity
   - Power management
   - CPU frequency scaling
   - Resource limits

4. **Continuous Improvement**
   - Performance regression testing
   - Latency distribution analysis
   - Resource utilization tracking
   - Competitive benchmarking
   - Strategy effectiveness metrics
   - Risk-adjusted returns
   - Capital efficiency metrics
   - Market impact analysis
