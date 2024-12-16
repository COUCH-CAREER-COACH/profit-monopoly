package network

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/michaelpento.lv/mevbot/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// mockDPDKManager implements network management interface for testing
type mockDPDKManager struct {
	running atomic.Bool
	cfg     *config.NetworkConfig
	logger  *zap.Logger
}

func newMockDPDKManager(cfg *config.NetworkConfig, logger *zap.Logger) (*mockDPDKManager, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}
	if logger == nil {
		return nil, errors.New("logger cannot be nil")
	}
	return &mockDPDKManager{
		cfg:    cfg,
		logger: logger,
	}, nil
}

func (m *mockDPDKManager) Initialize(ctx context.Context) error {
	if m.cfg.MaxPackets == 0 {
		return errors.New("invalid max packets configuration")
	}
	if m.cfg.Port == 0 {
		return errors.New("invalid port configuration")
	}

	// Simulate initialization delay
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	return nil
}

func (m *mockDPDKManager) Start() error {
	if m.running.Load() {
		return errors.New("network manager already running")
	}
	m.running.Store(true)
	m.logger.Info("DPDK network manager started")
	return nil
}

func (m *mockDPDKManager) Stop() error {
	if !m.running.Load() {
		return errors.New("network manager not running")
	}
	m.running.Store(false)
	m.logger.Info("DPDK network manager stopped")
	return nil
}

func (m *mockDPDKManager) IsRunning() bool {
	return m.running.Load()
}

func TestDPDKConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name        string
		configMod   func(*config.NetworkConfig)
		expectError bool
	}{
		{
			name: "valid_config",
			configMod: func(cfg *config.NetworkConfig) {
				cfg.Interface = "eth0"
				cfg.Port = 8545
				cfg.MaxPackets = 32
			},
			expectError: false,
		},
		{
			name: "invalid_max_packets",
			configMod: func(cfg *config.NetworkConfig) {
				cfg.Interface = "eth0"
				cfg.Port = 8545
				cfg.MaxPackets = 0
			},
			expectError: true,
		},
		{
			name: "invalid_port",
			configMod: func(cfg *config.NetworkConfig) {
				cfg.Interface = "eth0"
				cfg.Port = 0
				cfg.MaxPackets = 32
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.NetworkConfig{}
			tt.configMod(cfg)

			manager, err := newMockDPDKManager(cfg, logger)
			require.NoError(t, err)
			require.NotNil(t, manager)

			ctx := context.Background()
			err = manager.Initialize(ctx)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			err = manager.Start()
			require.NoError(t, err)
			assert.True(t, manager.IsRunning())

			err = manager.Stop()
			require.NoError(t, err)
			assert.False(t, manager.IsRunning())
		})
	}
}

func TestNetworkManagerLifecycle(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.NetworkConfig{
		Interface:  "eth0",
		Port:       8545,
		MaxPackets: 32,
	}

	manager, err := newMockDPDKManager(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, manager)

	// Test double start
	err = manager.Start()
	require.NoError(t, err)
	assert.True(t, manager.IsRunning())

	err = manager.Start()
	require.Error(t, err)
	assert.True(t, manager.IsRunning())

	// Test double stop
	err = manager.Stop()
	require.NoError(t, err)
	assert.False(t, manager.IsRunning())

	err = manager.Stop()
	require.Error(t, err)
	assert.False(t, manager.IsRunning())
}

func TestNetworkManagerInitialization(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.NetworkConfig{
		Interface:  "eth0",
		Port:       8545,
		MaxPackets: 32,
	}

	// Test nil config
	manager, err := newMockDPDKManager(nil, logger)
	require.Error(t, err)
	require.Nil(t, manager)

	// Test nil logger
	manager, err = newMockDPDKManager(cfg, nil)
	require.Error(t, err)
	require.Nil(t, manager)

	// Test context cancellation
	manager, err = newMockDPDKManager(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, manager)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = manager.Initialize(ctx)
	require.Error(t, err)
}

func BenchmarkDPDKNetworkManager(b *testing.B) {
	logger := zap.NewNop()
	cfg := &config.NetworkConfig{
		Interface:  "eth0",
		Port:       8545,
		MaxPackets: 32,
	}

	manager, err := newMockDPDKManager(cfg, logger)
	require.NoError(b, err)
	require.NotNil(b, manager)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.Initialize(ctx)
		_ = manager.Start()
		_ = manager.Stop()
	}
}

func TestNetworkOptimizer(t *testing.T) {
	logger := zap.NewExample()
	ctx := context.Background()

	// Create test config
	cfg := DefaultConfig()
	cfg.RxQueueSize = 2048
	cfg.TxQueueSize = 2048
	cfg.EnableHugePages = true

	// Create optimizer
	optimizer, err := NewNetworkOptimizer(ctx, logger, cfg)
	require.NoError(t, err)
	require.NotNil(t, optimizer)
	defer optimizer.Cleanup()

	// Test queue optimization
	stats, err := optimizer.OptimizeRxQueue()
	require.NoError(t, err)
	assert.True(t, stats.PacketsProcessed > 0)
	assert.True(t, stats.DroppedPackets == 0)

	stats, err = optimizer.OptimizeTxQueue()
	require.NoError(t, err)
	assert.True(t, stats.PacketsProcessed > 0)
	assert.True(t, stats.DroppedPackets == 0)

	// Test metrics collection
	metrics := optimizer.GetPortMetrics()
	assert.NotNil(t, metrics)

	// Test huge pages
	enabled := optimizer.AreHugePagesEnabled()
	assert.True(t, enabled)
}

func BenchmarkNetworkOptimizer(b *testing.B) {
	logger := zap.NewExample()
	ctx := context.Background()

	// Create test config
	cfg := DefaultConfig()
	cfg.RxQueueSize = 2048
	cfg.TxQueueSize = 2048

	// Create optimizer
	optimizer, err := NewNetworkOptimizer(ctx, logger, cfg)
	require.NoError(b, err)
	defer optimizer.Cleanup()

	b.Run("BenchmarkRxQueueOptimization", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := optimizer.OptimizeRxQueue()
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("BenchmarkTxQueueOptimization", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := optimizer.OptimizeTxQueue()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
