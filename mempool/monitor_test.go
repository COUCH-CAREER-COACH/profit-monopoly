package mempool

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/michaelpento.lv/mevbot/config"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// mockEthClient implements ethclient.Client for testing
type mockEthClient struct {
	mu sync.RWMutex

	chainID     *big.Int
	blockNumber *big.Int
	blocks      map[common.Hash]*types.Block
	txs         map[common.Hash]*types.Transaction
	receipts    map[common.Hash]*types.Receipt

	// Error simulation
	shouldError bool
	errorMsg    string
}

func newMockEthClient() *mockEthClient {
	return &mockEthClient{
		chainID:     big.NewInt(1),
		blockNumber: big.NewInt(0),
		blocks:      make(map[common.Hash]*types.Block),
		txs:         make(map[common.Hash]*types.Transaction),
		receipts:    make(map[common.Hash]*types.Receipt),
	}
}

func (m *mockEthClient) setError(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldError = true
	m.errorMsg = msg
}

func (m *mockEthClient) clearError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldError = false
	m.errorMsg = ""
}

func (m *mockEthClient) ChainID(ctx context.Context) (*big.Int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return nil, errors.New(m.errorMsg)
	}
	return m.chainID, nil
}

func (m *mockEthClient) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return nil, errors.New(m.errorMsg)
	}

	if block, exists := m.blocks[hash]; exists {
		return block, nil
	}
	return nil, errors.New("block not found")
}

func (m *mockEthClient) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return nil, errors.New(m.errorMsg)
	}

	header := &types.Header{
		Number:     number,
		Time:       uint64(time.Now().Unix()),
		Difficulty: big.NewInt(1),
		GasLimit:   8000000,
	}
	return types.NewBlock(header, nil, nil, nil, nil), nil
}

func (m *mockEthClient) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return nil, errors.New(m.errorMsg)
	}

	return &types.Header{
		Number:     number,
		Time:       uint64(time.Now().Unix()),
		Difficulty: big.NewInt(1),
		GasLimit:   8000000,
	}, nil
}

func (m *mockEthClient) TransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return nil, false, errors.New(m.errorMsg)
	}

	if tx, exists := m.txs[hash]; exists {
		return tx, false, nil
	}
	return nil, false, errors.New("tx not found")
}

func (m *mockEthClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return nil, errors.New(m.errorMsg)
	}

	if receipt, exists := m.receipts[txHash]; exists {
		return receipt, nil
	}
	return nil, errors.New("receipt not found")
}

func (m *mockEthClient) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return 0, errors.New(m.errorMsg)
	}
	return 0, nil
}

func (m *mockEthClient) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return nil, errors.New(m.errorMsg)
	}
	return big.NewInt(1000000000), nil // 1 gwei
}

func (m *mockEthClient) EstimateGas(ctx context.Context, msg *types.Message) (uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return 0, errors.New(m.errorMsg)
	}
	return 21000, nil
}

func (m *mockEthClient) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldError {
		return errors.New(m.errorMsg)
	}

	m.txs[tx.Hash()] = tx
	return nil
}

func (m *mockEthClient) Close() {}

// createTestTransaction creates a test transaction for testing
func createTestTransaction(nonce uint64, gasPrice *big.Int) *types.Transaction {
	to := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	data := []byte("test transaction")
	gasLimit := uint64(21000)
	value := big.NewInt(0)
	return types.NewTransaction(nonce, to, value, gasLimit, gasPrice, data)
}

// TestCircuitBreaker verifies the circuit breaker functionality
func TestCircuitBreaker(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	tests := []struct {
		name           string
		errorThreshold uint64
		resetInterval  time.Duration
		cooldown       time.Duration
		numErrors      int
		expectTripped  bool
	}{
		{
			name:           "normal_operation",
			errorThreshold: 5,
			resetInterval:  time.Second,
			cooldown:       time.Second,
			numErrors:      3,
			expectTripped:  false,
		},
		{
			name:           "tripped_circuit",
			errorThreshold: 5,
			resetInterval:  time.Second,
			cooldown:       time.Second,
			numErrors:      6,
			expectTripped:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			breaker := NewCircuitBreaker(&config.CircuitBreakerConfig{
				Enabled:          true,
				ErrorThreshold:   tt.errorThreshold,
				ResetInterval:    tt.resetInterval,
				CooldownPeriod:   tt.cooldown,
				MinHealthyPeriod: time.Second,
			}, logger)
			require.NotNil(t, breaker)

			for i := 0; i < tt.numErrors; i++ {
				breaker.RecordError(errors.New("test error"))
			}

			assert.Equal(t, !tt.expectTripped, breaker.IsHealthy())
		})
	}
}

// TestMempoolMonitor verifies mempool monitoring functionality
func TestMempoolMonitor(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	tests := []struct {
		name          string
		injectErrors  bool
		expectMetrics bool
	}{
		{
			name:          "successful_monitoring",
			injectErrors:  false,
			expectMetrics: true,
		},
		{
			name:          "failed_monitoring",
			injectErrors:  true,
			expectMetrics: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newMockEthClient()
			cfg := &config.Config{
				Network: &config.NetworkConfig{
					Interface:  "eth0",
					Port:       8545,
					MaxPackets: 32,
				},
				CircuitBreaker: config.CircuitBreakerConfig{
					Enabled:          true,
					ErrorThreshold:   5,
					ResetInterval:    time.Second,
					CooldownPeriod:   time.Second,
					MinHealthyPeriod: time.Second,
				},
			}

			monitor := NewMempoolMonitor(cfg, client, logger)
			require.NotNil(t, monitor)

			if tt.injectErrors {
				client.setError("simulated error")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			txChan := monitor.Start(ctx)
			require.NotNil(t, txChan)

			// Give some time for monitoring
			time.Sleep(100 * time.Millisecond)

			monitor.Shutdown()
		})
	}
}

// BenchmarkCircuitBreaker measures circuit breaker performance
func BenchmarkCircuitBreaker(b *testing.B) {
	logger := zap.NewNop()
	breaker := NewCircuitBreaker(&config.CircuitBreakerConfig{
		Enabled:          true,
		ErrorThreshold:   5,
		ResetInterval:    time.Second,
		CooldownPeriod:   time.Second,
		MinHealthyPeriod: time.Second,
	}, logger)
	require.NotNil(b, breaker)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		breaker.RecordError(errors.New("test error"))
		_ = breaker.IsHealthy()
	}
}

// TestConcurrentTransactions verifies handling of multiple simultaneous transactions
func TestConcurrentTransactions(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	client := newMockEthClient()
	monitor, err := NewMempoolMonitor(&config.Config{
		Network: &config.NetworkConfig{
			Interface:  "eth0",
			Port:       8545,
			MaxPackets: 32,
		},
		CircuitBreaker: &config.CircuitBreakerConfig{
			Enabled:          true,
			ErrorThreshold:   5,
			ResetInterval:    time.Second,
			CooldownPeriod:   time.Second,
			MinHealthyPeriod: time.Second,
		},
	}, client, logger)
	require.NoError(t, err)
	require.NotNil(t, monitor)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	txChan := monitor.Start(ctx)
	require.NotNil(t, txChan)

	// Launch multiple goroutines to simulate concurrent transactions
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tx := createTestTransaction(uint64(i), big.NewInt(20000000000))
			client.txs[tx.Hash()] = tx
		}(i)
	}

	wg.Wait()
	monitor.Stop()
}

// TestMemoryLimits verifies memory usage and cleanup
func TestMemoryLimits(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	client := newMockEthClient()
	monitor, err := NewMempoolMonitor(&config.Config{
		Network: &config.NetworkConfig{
			Interface:  "eth0",
			Port:       8545,
			MaxPackets: 32,
			MaxMemory:  1024 * 1024, // 1MB limit
		},
		CircuitBreaker: &config.CircuitBreakerConfig{
			Enabled:          true,
			ErrorThreshold:   5,
			ResetInterval:    time.Second,
			CooldownPeriod:   time.Second,
			MinHealthyPeriod: time.Second,
		},
	}, client, logger)
	require.NoError(t, err)
	require.NotNil(t, monitor)

	ctx := context.Background()
	txChan := monitor.Start(ctx)
	require.NotNil(t, txChan)

	// Add transactions until we hit memory limit
	for i := 0; i < 1000; i++ {
		tx := createTestTransaction(uint64(i), big.NewInt(20000000000))
		client.txs[tx.Hash()] = tx
		if !monitor.IsHealthy() {
			break
		}
	}

	// Verify cleanup occurs
	time.Sleep(100 * time.Millisecond)
	assert.True(t, monitor.IsHealthy())

	monitor.Stop()
}

// TestNetworkLatency verifies behavior under different network conditions
func TestNetworkLatency(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	tests := []struct {
		name          string
		latency       time.Duration
		expectTimeout bool
	}{
		{
			name:          "low_latency",
			latency:       50 * time.Millisecond,
			expectTimeout: false,
		},
		{
			name:          "high_latency",
			latency:       2 * time.Second,
			expectTimeout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newMockEthClient()
			monitor, err := NewMempoolMonitor(&config.Config{
				Network: &config.NetworkConfig{
					Interface:    "eth0",
					Port:         8545,
					MaxPackets:   32,
					ReadTimeout:  time.Second,
					WriteTimeout: time.Second,
				},
				CircuitBreaker: &config.CircuitBreakerConfig{
					Enabled:          true,
					ErrorThreshold:   5,
					ResetInterval:    time.Second,
					CooldownPeriod:   time.Second,
					MinHealthyPeriod: time.Second,
				},
			}, client, logger)
			require.NoError(t, err)
			require.NotNil(t, monitor)

			ctx := context.Background()
			txChan := monitor.Start(ctx)
			require.NotNil(t, txChan)

			// Simulate network latency
			tx := createTestTransaction(0, big.NewInt(20000000000))
			time.Sleep(tt.latency)
			client.txs[tx.Hash()] = tx

			if tt.expectTimeout {
				assert.False(t, monitor.IsHealthy())
			} else {
				assert.True(t, monitor.IsHealthy())
			}

			monitor.Stop()
		})
	}
}

// TestCircuitBreakerEdgeCases verifies circuit breaker behavior in edge cases
func TestCircuitBreakerEdgeCases(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	tests := []struct {
		name           string
		errorPattern   []bool // true = error, false = success
		expectHealthy  bool
		cooldownChecks int
	}{
		{
			name:           "error_recovery",
			errorPattern:   []bool{true, true, false, false, false},
			expectHealthy:  true,
			cooldownChecks: 1,
		},
		{
			name:           "intermittent_errors",
			errorPattern:   []bool{true, false, true, false, true},
			expectHealthy:  true,
			cooldownChecks: 3,
		},
		{
			name:           "rapid_errors",
			errorPattern:   []bool{true, true, true, true, true},
			expectHealthy:  false,
			cooldownChecks: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newMockEthClient()
			monitor, err := NewMempoolMonitor(&config.Config{
				Network: &config.NetworkConfig{
					Interface:  "eth0",
					Port:       8545,
					MaxPackets: 32,
				},
				CircuitBreaker: &config.CircuitBreakerConfig{
					Enabled:          true,
					ErrorThreshold:   3,
					ResetInterval:    100 * time.Millisecond,
					CooldownPeriod:   100 * time.Millisecond,
					MinHealthyPeriod: 100 * time.Millisecond,
				},
			}, client, logger)
			require.NoError(t, err)
			require.NotNil(t, monitor)

			ctx := context.Background()
			txChan := monitor.Start(ctx)
			require.NotNil(t, txChan)

			for _, shouldError := range tt.errorPattern {
				if shouldError {
					client.setError("simulated error")
				} else {
					client.clearError()
				}
				time.Sleep(50 * time.Millisecond)
			}

			time.Sleep(200 * time.Millisecond)
			assert.Equal(t, tt.expectHealthy, monitor.IsHealthy())

			monitor.Stop()
		})
	}
}

// BenchmarkHighLoad measures performance under high transaction load
func BenchmarkHighLoad(b *testing.B) {
	logger := zaptest.NewLogger(b)
	client := newMockEthClient()
	monitor, _ := NewMempoolMonitor(&config.Config{
		Network: &config.NetworkConfig{
			Interface:  "eth0",
			Port:       8545,
			MaxPackets: 32,
		},
		CircuitBreaker: &config.CircuitBreakerConfig{
			Enabled:          true,
			ErrorThreshold:   5,
			ResetInterval:    time.Second,
			CooldownPeriod:   time.Second,
			MinHealthyPeriod: time.Second,
		},
	}, client, logger)

	ctx := context.Background()
	txChan := monitor.Start(ctx)

	b.ResetTimer()

	b.Run("HighThroughput", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tx := createTestTransaction(uint64(i), big.NewInt(20000000000))
			client.txs[tx.Hash()] = tx
		}
	})

	b.Run("ConcurrentLoad", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				tx := createTestTransaction(uint64(i), big.NewInt(20000000000))
				client.txs[tx.Hash()] = tx
				i++
			}
		})
	})

	monitor.Stop()
}
