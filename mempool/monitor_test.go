package mempool

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/michaelpento.lv/mevbot/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

type mockEthClient struct {
	mock.Mock
	*ethclient.Client
}

func (m *mockEthClient) SubscribePendingTransactions(ctx context.Context, ch chan<- common.Hash) (ethereum.Subscription, error) {
	args := m.Called(ctx, ch)
	return args.Get(0).(ethereum.Subscription), args.Error(1)
}

func (m *mockEthClient) TransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	args := m.Called(ctx, hash)
	return args.Get(0).(*types.Transaction), args.Bool(1), args.Error(2)
}

func TestNewMempoolMonitor(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		MempoolConfig: struct {
			MaxPendingTx       int     `json:"max_pending_tx"`
			BlockBufferSize    int     `json:"block_buffer_size"`
			MinProfitThreshold float64 `json:"min_profit_threshold"`
			GasBoostFactor    float64 `json:"gas_boost_factor"`
			DataDir           string  `json:"data_dir"`
			Workers           int     `json:"workers"`
		}{
			MaxPendingTx:       10000,
			BlockBufferSize:    50,
			MinProfitThreshold: 0.01,
			GasBoostFactor:     1.2,
			DataDir:            "/tmp/test_mempool.mmap",
			Workers:            4,
		},
		RateLimit:  100.0,
		RateBurst:  10,
		MaxGasPrice: big.NewInt(100e9), // 100 Gwei
	}

	// Create logger
	logger, err := zap.NewDevelopment()
	assert.NoError(t, err)

	// Create mock client
	client := new(mockEthClient)

	// Create monitor
	monitor, err := NewMempoolMonitor(cfg, client, logger)
	assert.NoError(t, err)
	assert.NotNil(t, monitor)

	// Verify monitor fields
	assert.Equal(t, cfg, monitor.cfg)
	assert.Equal(t, client, monitor.client)
	assert.Equal(t, logger, monitor.logger)
	assert.NotNil(t, monitor.txChan)
	assert.NotNil(t, monitor.txPool)
	assert.NotNil(t, monitor.workers)
	assert.NotNil(t, monitor.limiter)
	assert.NotNil(t, monitor.cache)
	assert.NotNil(t, monitor.breaker)
	assert.NotNil(t, monitor.netManager)
}

func TestMempoolMonitor_ProcessTransaction(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		MempoolConfig: struct {
			MaxPendingTx       int     `json:"max_pending_tx"`
			BlockBufferSize    int     `json:"block_buffer_size"`
			MinProfitThreshold float64 `json:"min_profit_threshold"`
			GasBoostFactor    float64 `json:"gas_boost_factor"`
			DataDir           string  `json:"data_dir"`
			Workers           int     `json:"workers"`
		}{
			MaxPendingTx:       10000,
			BlockBufferSize:    50,
			MinProfitThreshold: 0.01,
			GasBoostFactor:     1.2,
			DataDir:            "/tmp/test_mempool.mmap",
			Workers:            4,
		},
		RateLimit:  100.0,
		RateBurst:  10,
		MaxGasPrice: big.NewInt(100e9), // 100 Gwei
	}

	// Create logger
	logger, err := zap.NewDevelopment()
	assert.NoError(t, err)

	// Create mock client
	client := new(mockEthClient)

	// Create test transaction
	tx := types.NewTransaction(
		0,                          // nonce
		common.Address{},           // to
		big.NewInt(0),             // value
		21000,                     // gas limit
		big.NewInt(50e9),          // gas price (50 Gwei)
		[]byte{},                  // data
	)

	// Set up mock expectations
	txHash := tx.Hash()
	client.On("TransactionByHash", mock.Anything, txHash).Return(tx, true, nil)

	// Create monitor
	monitor, err := NewMempoolMonitor(cfg, client, logger)
	assert.NoError(t, err)

	// Process transaction
	transaction := &Transaction{
		Transaction: tx,
		FirstSeen:  time.Now(),
		GasPrice:   tx.GasPrice(),
		Priority:   calculatePriority(tx),
	}

	err = monitor.processPendingTx(transaction)
	assert.NoError(t, err)

	// Verify mock expectations
	client.AssertExpectations(t)
}

func TestMempoolMonitor_IsRelevantTransaction(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		MempoolConfig: struct {
			MaxPendingTx       int     `json:"max_pending_tx"`
			BlockBufferSize    int     `json:"block_buffer_size"`
			MinProfitThreshold float64 `json:"min_profit_threshold"`
			GasBoostFactor    float64 `json:"gas_boost_factor"`
			DataDir           string  `json:"data_dir"`
			Workers           int     `json:"workers"`
		}{
			MaxPendingTx:       10000,
			BlockBufferSize:    50,
			MinProfitThreshold: 0.01,
			GasBoostFactor:     1.2,
			DataDir:            "/tmp/test_mempool.mmap",
			Workers:            4,
		},
		RateLimit:  100.0,
		RateBurst:  10,
		MaxGasPrice: big.NewInt(100e9), // 100 Gwei
	}

	// Create logger
	logger, err := zap.NewDevelopment()
	assert.NoError(t, err)

	// Create mock client
	client := new(mockEthClient)

	// Create monitor
	monitor, err := NewMempoolMonitor(cfg, client, logger)
	assert.NoError(t, err)

	// Test cases
	testCases := []struct {
		name     string
		tx       *Transaction
		expected bool
	}{
		{
			name: "Valid transaction",
			tx: &Transaction{
				Transaction: types.NewTransaction(
					0,                          // nonce
					common.Address{},           // to
					big.NewInt(0),             // value
					21000,                     // gas limit
					big.NewInt(50e9),          // gas price (50 Gwei)
					[]byte{},                  // data
				),
				FirstSeen: time.Now(),
				GasPrice:  big.NewInt(50e9),
				Priority:  1.0,
			},
			expected: true,
		},
		{
			name: "Gas price too high",
			tx: &Transaction{
				Transaction: types.NewTransaction(
					0,                          // nonce
					common.Address{},           // to
					big.NewInt(0),             // value
					21000,                     // gas limit
					big.NewInt(150e9),         // gas price (150 Gwei)
					[]byte{},                  // data
				),
				FirstSeen: time.Now(),
				GasPrice:  big.NewInt(150e9),
				Priority:  1.0,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := monitor.isRelevantTransaction(tc.tx)
			assert.Equal(t, tc.expected, result)
		})
	}
}
