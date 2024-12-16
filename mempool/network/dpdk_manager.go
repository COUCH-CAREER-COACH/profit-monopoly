package network

import (
	"context"
	"net"
	"sync"

	"go.uber.org/zap"
)

// DPDKManager implements NetworkManager using DPDK
type DPDKManager struct {
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *zap.Logger
	config  *Config
	mu      sync.RWMutex
	dpdk    *NetworkOptimizer
	enabled bool
}

// NewDPDKManager creates a new DPDK manager
func NewDPDKManager(ctx context.Context, logger *zap.Logger, config *Config) (*DPDKManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(ctx)
	
	manager := &DPDKManager{
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		config:  config,
		enabled: false,
	}

	// Try to initialize DPDK
	if optimizer, err := NewNetworkOptimizer(ctx, logger, config); err == nil {
		manager.dpdk = optimizer
		manager.enabled = true
		logger.Info("DPDK initialized successfully")
	} else {
		logger.Warn("Failed to initialize DPDK, falling back to standard networking", zap.Error(err))
	}

	return manager, nil
}

// IsEnabled returns whether DPDK is enabled
func (m *DPDKManager) IsEnabled() bool {
	return m.enabled
}

// OptimizeConnection applies DPDK optimizations to a connection
func (m *DPDKManager) OptimizeConnection(conn net.Conn) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled || m.dpdk == nil {
		return nil // No-op when DPDK is not enabled
	}

	// Apply DPDK-specific optimizations
	return m.dpdk.OptimizeConn(conn)
}

// Cleanup releases all resources
func (m *DPDKManager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.enabled && m.dpdk != nil {
		if err := m.dpdk.Cleanup(); err != nil {
			m.logger.Error("Failed to cleanup DPDK", zap.Error(err))
			return err
		}
	}

	m.cancel()
	return nil
}
