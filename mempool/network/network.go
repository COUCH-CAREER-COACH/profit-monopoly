// Package network provides network optimization utilities for the MEV bot
package network

import (
	"context"
	"errors"
	"fmt"
	"github.com/michaelpento.lv/mevbot/utils/metrics"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	ErrNotTCPConn = errors.New("connection is not TCP")
)

// Config holds the network optimizer configuration
type Config struct {
	DialTimeout      time.Duration
	KeepAlivePeriod  time.Duration
	ReconnectBackoff time.Duration
	MaxReconnects    int
	MetricsInterval  time.Duration
	ReadBufferSize   int
	WriteBufferSize  int
	CPUAffinity      []int
	RxQueueSize      int
	TxQueueSize      int
	EnableHugePages  bool
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		DialTimeout:      time.Second * 10,
		KeepAlivePeriod:  time.Second * 30,
		ReconnectBackoff: time.Second * 5,
		MaxReconnects:    3,
		MetricsInterval:  time.Second * 10,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		CPUAffinity:      []int{0},
		RxQueueSize:      1024,
		TxQueueSize:      1024,
		EnableHugePages:  false,
	}
}

// QueueStats holds queue statistics
type QueueStats struct {
	PacketsProcessed uint64
	DroppedPackets   uint64
	Latency         time.Duration
}

// NetworkOptimizer represents a high-performance network interface
type NetworkOptimizer struct {
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *zap.Logger
	config  *Config
	mu      sync.RWMutex
	metrics *metrics.NetworkMetrics

	rxStats *QueueStats
	txStats *QueueStats
}

// NewNetworkOptimizer creates a new network optimizer instance
func NewNetworkOptimizer(ctx context.Context, logger *zap.Logger, config *Config) (*NetworkOptimizer, error) {
	if logger == nil {
		return nil, errors.New("logger cannot be nil")
	}
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}
	if config.RxQueueSize <= 0 || config.TxQueueSize <= 0 {
		return nil, fmt.Errorf("invalid queue sizes: rx=%d, tx=%d", config.RxQueueSize, config.TxQueueSize)
	}

	ctx, cancel := context.WithCancel(ctx)
	
	metrics := metrics.NewNetworkMetrics("network.optimizer")
	if metrics == nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize network metrics: %w", errors.New("metrics initialization failed"))
	}
	
	optimizer := &NetworkOptimizer{
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		config:  config,
		metrics: metrics,
		rxStats: &QueueStats{},
		txStats: &QueueStats{},
	}

	// Start metrics reporting
	go optimizer.reportMetrics()

	return optimizer, nil
}

// Cleanup releases all resources
func (n *NetworkOptimizer) Cleanup() error {
	n.cancel()
	return nil
}

// reportMetrics reports metrics periodically
func (n *NetworkOptimizer) reportMetrics() {
	ticker := time.NewTicker(n.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.mu.RLock()
			if n.rxStats.PacketsProcessed > 0 {
				n.metrics.RxPackets.Add(float64(n.rxStats.PacketsProcessed))
			}
			if n.txStats.PacketsProcessed > 0 {
				n.metrics.TxPackets.Add(float64(n.txStats.PacketsProcessed))
			}
			if n.rxStats.DroppedPackets > 0 || n.txStats.DroppedPackets > 0 {
				n.metrics.Errors.Add(float64(n.rxStats.DroppedPackets + n.txStats.DroppedPackets))
			}
			n.mu.RUnlock()
		}
	}
}

// OptimizeRxQueue optimizes the receive queue
func (n *NetworkOptimizer) OptimizeRxQueue() (*QueueStats, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Simulate queue optimization
	n.rxStats.PacketsProcessed++
	n.rxStats.Latency = time.Microsecond * 100
	return n.rxStats, nil
}

// OptimizeTxQueue optimizes the transmit queue
func (n *NetworkOptimizer) OptimizeTxQueue() (*QueueStats, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Simulate queue optimization
	n.txStats.PacketsProcessed++
	n.txStats.Latency = time.Microsecond * 100
	return n.txStats, nil
}

// GetPortMetrics returns the network metrics
func (n *NetworkOptimizer) GetPortMetrics() *metrics.NetworkMetrics {
	return n.metrics
}

// SetupHugePages sets up huge pages
func (n *NetworkOptimizer) SetupHugePages() error {
	// Simulate huge pages setup
	if !n.config.EnableHugePages {
		return errors.New("huge pages not enabled in config")
	}
	return nil
}

// AreHugePagesEnabled returns whether huge pages are enabled
func (n *NetworkOptimizer) AreHugePagesEnabled() bool {
	return n.config.EnableHugePages
}

// OptimizeConn optimizes a network connection
func (n *NetworkOptimizer) OptimizeConn(conn net.Conn) error {
	if conn == nil {
		return fmt.Errorf("connection cannot be nil: %w", errors.New("invalid connection"))
	}

	n.metrics.Connections.Inc()

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		n.metrics.Errors.Inc()
		return fmt.Errorf("connection is not TCP: %w", ErrNotTCPConn)
	}

	// Set keep alive
	if err := tcpConn.SetKeepAlive(true); err != nil {
		n.metrics.Errors.Inc()
		return fmt.Errorf("failed to set keep alive: %w", err)
	}

	if err := tcpConn.SetKeepAlivePeriod(n.config.KeepAlivePeriod); err != nil {
		n.metrics.Errors.Inc()
		return fmt.Errorf("failed to set keep alive period: %w", err)
	}

	// Set buffer sizes
	if err := tcpConn.SetReadBuffer(n.config.ReadBufferSize); err != nil {
		n.metrics.Errors.Inc()
		return fmt.Errorf("failed to set read buffer size: %w", err)
	}

	if err := tcpConn.SetWriteBuffer(n.config.WriteBufferSize); err != nil {
		n.metrics.Errors.Inc()
		return fmt.Errorf("failed to set write buffer size: %w", err)
	}

	return nil
}

// Send sends data over the connection with metrics
func (n *NetworkOptimizer) Send(conn net.Conn, data []byte) error {
	if conn == nil {
		return fmt.Errorf("connection cannot be nil: %w", errors.New("invalid connection"))
	}
	if len(data) == 0 {
		return fmt.Errorf("data cannot be empty: %w", errors.New("invalid data"))
	}

	start := time.Now()
	_, err := conn.Write(data)
	duration := time.Since(start)

	n.metrics.SendLatency.Observe(duration.Seconds())

	if err != nil {
		n.metrics.Errors.Inc()
		return fmt.Errorf("failed to send data: %w", err)
	}

	n.metrics.BytesSent.Add(float64(len(data)))
	return nil
}

// Receive receives data from the connection with metrics
func (n *NetworkOptimizer) Receive(conn net.Conn, buffer []byte) (int, error) {
	if conn == nil {
		return 0, fmt.Errorf("connection cannot be nil: %w", errors.New("invalid connection"))
	}
	if len(buffer) == 0 {
		return 0, fmt.Errorf("buffer cannot be empty: %w", errors.New("invalid buffer"))
	}

	start := time.Now()
	bytesRead, err := conn.Read(buffer)
	duration := time.Since(start)

	n.metrics.RecvLatency.Observe(duration.Seconds())

	if err != nil {
		n.metrics.Errors.Inc()
		return 0, fmt.Errorf("failed to receive data: %w", err)
	}

	n.metrics.BytesRecv.Add(float64(bytesRead))
	return bytesRead, nil
}
