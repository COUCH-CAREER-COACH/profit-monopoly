package network

import (
	"context"
	"fmt"
	"github.com/michaelpento.lv/mevbot/utils/metrics"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"go.uber.org/zap"
)

// StandardNetworkManager implements basic network management
type StandardNetworkManager struct {
	logger  *zap.Logger
	config  *NetworkConfig
	conn    net.PacketConn
	mutex   sync.RWMutex
	done    chan struct{}
	metrics *metrics.NetworkMetrics
	indexer *MempoolIndexer
	ctx     context.Context

	rxStats *QueueStats
	txStats *QueueStats
}

// NewStandardNetworkManager creates a new standard network manager
func NewStandardNetworkManager(config *NetworkConfig) NetworkManager {
	ctx := context.Background()
	return &StandardNetworkManager{
		config:  config,
		ctx:     ctx,
		done:    make(chan struct{}),
		metrics: metrics.NewNetworkMetrics("network.standard"),
		indexer: NewMempoolIndexer(ctx, zap.L(), 10000), // Configure max txs as needed
		rxStats: &QueueStats{},
		txStats: &QueueStats{},
	}
}

// Initialize prepares the network manager
func (m *StandardNetworkManager) Initialize(ctx context.Context) error {
	if m.config == nil {
		return fmt.Errorf("network config is nil")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.ctx = ctx

	// Set up socket with optimized buffer sizes
	conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", m.config.Port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", m.config.Port, err)
	}

	if udpConn, ok := conn.(*net.UDPConn); ok {
		udpConn.SetReadBuffer(8 * 1024 * 1024)  // 8MB read buffer
		udpConn.SetWriteBuffer(8 * 1024 * 1024) // 8MB write buffer
	}

	m.conn = conn
	return nil
}

// Start begins network operations
func (m *StandardNetworkManager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.conn == nil {
		return fmt.Errorf("connection not initialized")
	}

	// Start background monitoring
	go m.monitorMetrics()

	return nil
}

// monitorMetrics periodically updates network metrics
func (m *StandardNetworkManager) monitorMetrics() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateMetrics()
		}
	}
}

func (m *StandardNetworkManager) updateMetrics() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.indexer != nil {
		m.metrics.TxCount.Set(float64(m.indexer.txQueue.Len()))
		m.metrics.TxEvictions.Set(float64(len(m.indexer.txByHash)))
		m.metrics.TxLookups.Set(float64(len(m.indexer.txByNonce)))
	}
}

// Stop halts network operations
func (m *StandardNetworkManager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.conn != nil {
		close(m.done)
		return m.conn.Close()
	}
	return nil
}

// Read reads data from the network
func (m *StandardNetworkManager) Read(p []byte) (n int, err error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.conn == nil {
		return 0, fmt.Errorf("not connected")
	}

	start := time.Now()
	n, _, err = m.conn.ReadFrom(p)
	duration := time.Since(start)

	m.metrics.RecvLatency.Observe(duration.Seconds())
	if err != nil {
		m.metrics.Errors.Inc()
		return 0, fmt.Errorf("failed to read: %w", err)
	}

	m.metrics.BytesRecv.Add(float64(n))
	return n, nil
}

// Write writes data to the network
func (m *StandardNetworkManager) Write(p []byte) (n int, err error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.conn == nil {
		return 0, fmt.Errorf("not connected")
	}

	start := time.Now()
	n, err = m.conn.WriteTo(p, nil)
	duration := time.Since(start)

	m.metrics.SendLatency.Observe(duration.Seconds())
	if err != nil {
		m.metrics.Errors.Inc()
		return 0, fmt.Errorf("failed to write: %w", err)
	}

	m.metrics.BytesSent.Add(float64(n))
	return n, nil
}

// ReadPacket reads a packet from the network
func (m *StandardNetworkManager) ReadPacket() ([]byte, error) {
	buffer := make([]byte, 65536) // Standard MTU size
	n, err := m.Read(buffer)
	if err != nil {
		return nil, err
	}
	return buffer[:n], nil
}

// WritePacket writes a packet to the network
func (m *StandardNetworkManager) WritePacket(packet []byte) error {
	_, err := m.Write(packet)
	return err
}

// ProcessTransaction processes a transaction
func (m *StandardNetworkManager) ProcessTransaction(tx *types.Transaction) error {
	if tx == nil {
		return fmt.Errorf("nil transaction")
	}

	m.indexer.AddTransaction(tx)

	// Get effective gas price for metrics
	if price := getEffectiveGasPrice(tx); price != nil {
		m.metrics.GasPrice.Observe(float64(price.Uint64()))
	}

	return nil
}

// GetMetrics returns network metrics
func (m *StandardNetworkManager) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := map[string]interface{}{
		"rx_stats": m.rxStats,
		"tx_stats": m.txStats,
	}

	// Add indexer stats if available
	if m.indexer != nil {
		stats["tx_count"] = m.indexer.txQueue.Len()
		stats["tx_by_hash"] = len(m.indexer.txByHash)
		stats["tx_by_nonce"] = len(m.indexer.txByNonce)
	}

	return stats
}

// Close releases resources
func (m *StandardNetworkManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.conn != nil {
		m.indexer.Cleanup()
		close(m.done)
		return m.conn.Close()
	}
	return nil
}

// OptimizeRxQueue optimizes the receive queue
func (m *StandardNetworkManager) OptimizeRxQueue() (*QueueStats, error) {
	if m.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	if udpConn, ok := m.conn.(*net.UDPConn); ok {
		udpConn.SetReadBuffer(16 * 1024 * 1024) // Increase to 16MB for better performance
	}

	return m.rxStats, nil
}

// OptimizeTxQueue optimizes the transmit queue
func (m *StandardNetworkManager) OptimizeTxQueue() (*QueueStats, error) {
	if m.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	if udpConn, ok := m.conn.(*net.UDPConn); ok {
		udpConn.SetWriteBuffer(16 * 1024 * 1024) // Increase to 16MB for better performance
	}

	return m.txStats, nil
}

// GetPortMetrics returns network metrics
func (m *StandardNetworkManager) GetPortMetrics() *metrics.NetworkMetrics {
	return m.metrics
}

// OptimizeConn optimizes a network connection
func (m *StandardNetworkManager) OptimizeConn(conn net.Conn) error {
	if conn == nil {
		return fmt.Errorf("nil connection")
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)   // Disable Nagle's algorithm
		tcpConn.SetKeepAlive(true) // Enable keep-alive
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetLinger(0) // Don't wait on close
		tcpConn.SetReadBuffer(8 * 1024 * 1024)
		tcpConn.SetWriteBuffer(8 * 1024 * 1024)
	}

	return nil
}

// Send sends data over a connection
func (m *StandardNetworkManager) Send(conn net.Conn, data []byte) error {
	if conn == nil {
		return fmt.Errorf("nil connection")
	}

	start := time.Now()
	_, err := conn.Write(data)
	duration := time.Since(start)

	m.metrics.SendLatency.Observe(duration.Seconds())
	if err != nil {
		m.metrics.Errors.Inc()
		return fmt.Errorf("send error: %w", err)
	}

	m.metrics.BytesSent.Add(float64(len(data)))
	return nil
}

// Receive receives data from a connection
func (m *StandardNetworkManager) Receive(conn net.Conn, buffer []byte) (int, error) {
	if conn == nil {
		return 0, fmt.Errorf("nil connection")
	}

	start := time.Now()
	n, err := conn.Read(buffer)
	duration := time.Since(start)

	m.metrics.RecvLatency.Observe(duration.Seconds())
	if err != nil {
		m.metrics.Errors.Inc()
		return 0, fmt.Errorf("receive error: %w", err)
	}

	m.metrics.BytesRecv.Add(float64(n))
	return n, nil
}

// SetupHugePages attempts to enable huge pages for memory allocation
func (m *StandardNetworkManager) SetupHugePages() error {
	// TODO: Implement huge pages setup
	return fmt.Errorf("huge pages not implemented")
}

// AreHugePagesEnabled checks if huge pages are enabled
func (m *StandardNetworkManager) AreHugePagesEnabled() bool {
	return false // TODO: Implement huge pages check
}
