package dpdk

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Config contains network configuration
type Config struct {
	Interface    string
	NumRxQueues  uint
	NumTxQueues  uint
	PollTimeout  time.Duration
}

// Stats contains network statistics
type Stats struct {
	RxPackets uint64
	TxPackets uint64
	RxBytes   uint64
	TxBytes   uint64
	RxDrops   uint64
	TxDrops   uint64
	LatencyNs uint64
}

// DPDKManager implements network optimization
type DPDKManager struct {
	logger *zap.Logger
	config *Config
	conn   net.PacketConn
	stats  struct {
		rxPackets atomic.Uint64
		txPackets atomic.Uint64
		rxBytes   atomic.Uint64
		txBytes   atomic.Uint64
		rxDrops   atomic.Uint64
		txDrops   atomic.Uint64
		latencyNs atomic.Uint64
	}
	wg   sync.WaitGroup
	done chan struct{}
}

// NewDPDKManager creates a new network manager
func NewDPDKManager(logger *zap.Logger, config *Config) (*DPDKManager, error) {
	if config == nil {
		config = &Config{
			Interface:    "eth0",
			NumRxQueues:  4,
			NumTxQueues:  4,
			PollTimeout:  time.Millisecond,
		}
	}

	return &DPDKManager{
		logger: logger,
		config: config,
		done:   make(chan struct{}),
	}, nil
}

// Start starts the network manager
func (d *DPDKManager) Start(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return err
	}
	d.conn = conn

	d.wg.Add(1)
	go d.collectStats(ctx)

	return nil
}

// Stop stops the network manager
func (d *DPDKManager) Stop() error {
	close(d.done)
	if d.conn != nil {
		d.conn.Close()
	}
	d.wg.Wait()
	return nil
}

// collectStats collects and logs network statistics
func (d *DPDKManager) collectStats(ctx context.Context) {
	defer d.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.done:
			return
		case <-ticker.C:
			d.logger.Info("Network statistics",
				zap.Uint64("rx_packets", d.stats.rxPackets.Load()),
				zap.Uint64("tx_packets", d.stats.txPackets.Load()),
				zap.Uint64("rx_bytes", d.stats.rxBytes.Load()),
				zap.Uint64("tx_bytes", d.stats.txBytes.Load()),
				zap.Uint64("rx_drops", d.stats.rxDrops.Load()),
				zap.Uint64("tx_drops", d.stats.txDrops.Load()),
				zap.Duration("latency", time.Duration(d.stats.latencyNs.Load())),
			)
		}
	}
}

// GetStats returns current network statistics
func (d *DPDKManager) GetStats() Stats {
	return Stats{
		RxPackets: d.stats.rxPackets.Load(),
		TxPackets: d.stats.txPackets.Load(),
		RxBytes:   d.stats.rxBytes.Load(),
		TxBytes:   d.stats.txBytes.Load(),
		RxDrops:   d.stats.rxDrops.Load(),
		TxDrops:   d.stats.txDrops.Load(),
		LatencyNs: d.stats.latencyNs.Load(),
	}
}

// IncrementRxPackets increments received packets counter
func (d *DPDKManager) IncrementRxPackets() {
	d.stats.rxPackets.Add(1)
}

// IncrementTxPackets increments transmitted packets counter
func (d *DPDKManager) IncrementTxPackets() {
	d.stats.txPackets.Add(1)
}

// AddRxBytes adds received bytes
func (d *DPDKManager) AddRxBytes(n uint64) {
	d.stats.rxBytes.Add(n)
}

// AddTxBytes adds transmitted bytes
func (d *DPDKManager) AddTxBytes(n uint64) {
	d.stats.txBytes.Add(n)
}

// IncrementRxDrops increments dropped received packets counter
func (d *DPDKManager) IncrementRxDrops() {
	d.stats.rxDrops.Add(1)
}

// IncrementTxDrops increments dropped transmitted packets counter
func (d *DPDKManager) IncrementTxDrops() {
	d.stats.txDrops.Add(1)
}

// UpdateLatency updates the latency measurement
func (d *DPDKManager) UpdateLatency(latency time.Duration) {
	d.stats.latencyNs.Store(uint64(latency.Nanoseconds()))
}
