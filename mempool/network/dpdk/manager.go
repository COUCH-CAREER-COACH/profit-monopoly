package dpdk

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/intel-go/nff-go/flow"
	"github.com/intel-go/nff-go/packet"
	"github.com/intel-go/nff-go/types"
	"go.uber.org/zap"
)

const (
	// DPDK configuration
	numRxRings    = 4              // Number of RX rings
	numTxRings    = 4              // Number of TX rings
	rxBurstSize   = 32             // Size of RX burst
	txBurstSize   = 32             // Size of TX burst
	mbufCacheSize = 256            // Size of mbuf cache
	maxMbufs      = 8192           // Maximum number of mbufs
	rxDescriptors = 1024           // Number of RX descriptors
	txDescriptors = 1024           // Number of TX descriptors

	// Performance tuning
	prefetchDistance = 4    // Number of packets to prefetch
	cachelineSize    = 64   // Size of CPU cache line
	hugepageSize     = 2048 // Size of huge pages in MB
)

// DPDKManager implements network optimization using DPDK
type DPDKManager struct {
	logger *zap.Logger
	config *Config

	// Flow processing
	rxFlow    *flow.Flow
	txFlow    *flow.Flow
	flowTable *flow.Table

	// Statistics
	stats struct {
		rxPackets atomic.Uint64
		txPackets atomic.Uint64
		rxBytes   atomic.Uint64
		txBytes   atomic.Uint64
		rxDrops   atomic.Uint64
		txDrops   atomic.Uint64
		latencyNs atomic.Uint64
	}

	// Connection tracking
	connTracker *connTracker

	// Lifecycle
	wg   sync.WaitGroup
	done chan struct{}
}

// Config contains DPDK configuration
type Config struct {
	Interface    string        // Network interface name
	NumRxQueues  uint         // Number of RX queues
	NumTxQueues  uint         // Number of TX queues
	RSSKey      []byte        // RSS hash key
	PollTimeout time.Duration // Polling timeout
}

// NewDPDKManager creates a new DPDK network manager
func NewDPDKManager(logger *zap.Logger, config *Config) (*DPDKManager, error) {
	// Initialize DPDK EAL (Environment Abstraction Layer)
	if err := flow.SystemInit(&flow.Config{
		CPUList:        "0-3",                     // Use first 4 cores
		HugepagesCount: [...]int{hugepageSize, 0}, // 2GB hugepages
		MbufNumber:     maxMbufs,
		MbufCacheSize:  mbufCacheSize,
		NoMLX5:         false, // Enable Mellanox driver
		PerfMonitor:    true,  // Enable performance monitoring
	}); err != nil {
		return nil, fmt.Errorf("failed to initialize DPDK: %w", err)
	}

	m := &DPDKManager{
		logger: logger,
		config: config,
		done:   make(chan struct{}),
		connTracker: newConnTracker(),
	}

	// Initialize flow processing
	if err := m.initializeFlows(); err != nil {
		return nil, fmt.Errorf("failed to initialize flows: %w", err)
	}

	return m, nil
}

// Start starts the DPDK network manager
func (m *DPDKManager) Start(ctx context.Context) error {
	// Pin flow processing threads to cores
	for i := 0; i < runtime.NumCPU(); i++ {
		m.wg.Add(1)
		go m.processFlows(i)
	}

	// Start statistics collection
	m.wg.Add(1)
	go m.collectStats(ctx)

	return nil
}

// Stop stops the DPDK network manager
func (m *DPDKManager) Stop() error {
	close(m.done)
	m.wg.Wait()

	// Stop flows
	if err := flow.SystemStop(); err != nil {
		return fmt.Errorf("failed to stop DPDK: %w", err)
	}

	return nil
}

// initializeFlows sets up DPDK flow processing
func (m *DPDKManager) initializeFlows() error {
	var err error

	// Create RX flow
	m.rxFlow, err = flow.SetReceiver(m.config.Interface)
	if err != nil {
		return fmt.Errorf("failed to create RX flow: %w", err)
	}

	// Create flow table for connection tracking
	m.flowTable = flow.NewTable()
	if err := m.flowTable.SetHashFunc(m.hashPacket); err != nil {
		return fmt.Errorf("failed to set hash function: %w", err)
	}

	// Set up flow processing pipeline
	m.rxFlow = flow.SetSplitter(m.rxFlow, m.classifyPacket, uint(types.MaxPacketFlows))
	
	// Handle different packet types
	ethFlow := m.rxFlow.GetFlow(0)
	ipv4Flow := m.rxFlow.GetFlow(1)
	ipv6Flow := m.rxFlow.GetFlow(2)

	// Process Ethernet packets
	flow.SetHandler(ethFlow, m.handleEthPacket, nil)

	// Process IPv4 packets
	flow.SetHandler(ipv4Flow, m.handleIPv4Packet, nil)

	// Process IPv6 packets
	flow.SetHandler(ipv6Flow, m.handleIPv6Packet, nil)

	// Create TX flow
	m.txFlow, err = flow.SetSender(m.config.Interface)
	if err != nil {
		return fmt.Errorf("failed to create TX flow: %w", err)
	}

	return nil
}

// processFlows processes network flows on a dedicated core
func (m *DPDKManager) processFlows(coreID int) {
	defer m.wg.Done()

	// Pin this goroutine to a specific core
	runtime.LockOSThread()
	if err := flow.SetAffinity(uint(coreID)); err != nil {
		m.logger.Error("Failed to set CPU affinity", zap.Error(err))
		return
	}

	for {
		select {
		case <-m.done:
			return
		default:
			if err := flow.ProcessAll(); err != nil {
				m.logger.Error("Error processing flows", zap.Error(err))
			}
		}
	}
}

// Packet classification and handling functions
func (m *DPDKManager) classifyPacket(pkt *packet.Packet, mask *[types.MaxPacketFlows]bool) {
	// Classify packet based on protocol
	ipv4, ipv6, _ := pkt.ParseAllKnownL3()
	if ipv4 != nil {
		mask[1] = true
	} else if ipv6 != nil {
		mask[2] = true
	} else {
		mask[0] = true // Ethernet
	}
}

func (m *DPDKManager) handleEthPacket(pkt *packet.Packet, context flow.UserContext) {
	// Process Ethernet packet
	m.stats.rxPackets.Add(1)
	m.stats.rxBytes.Add(uint64(pkt.GetPacketLen()))
}

func (m *DPDKManager) handleIPv4Packet(pkt *packet.Packet, context flow.UserContext) {
	// Process IPv4 packet
	ipv4 := pkt.GetIPv4()
	if ipv4 == nil {
		m.stats.rxDrops.Add(1)
		return
	}

	// Track connection
	conn := m.connTracker.track(
		net.IP(ipv4.SrcAddr[:]),
		net.IP(ipv4.DstAddr[:]),
		uint16(ipv4.SrcPort),
		uint16(ipv4.DstPort),
	)

	// Update statistics
	m.stats.rxPackets.Add(1)
	m.stats.rxBytes.Add(uint64(pkt.GetPacketLen()))
	
	// Measure latency
	if conn != nil {
		latency := time.Since(conn.lastSeen)
		m.stats.latencyNs.Store(uint64(latency.Nanoseconds()))
	}
}

func (m *DPDKManager) handleIPv6Packet(pkt *packet.Packet, context flow.UserContext) {
	// Process IPv6 packet
	ipv6 := pkt.GetIPv6()
	if ipv6 == nil {
		m.stats.rxDrops.Add(1)
		return
	}

	// Update statistics
	m.stats.rxPackets.Add(1)
	m.stats.rxBytes.Add(uint64(pkt.GetPacketLen()))
}

func (m *DPDKManager) hashPacket(pkt *packet.Packet, context flow.UserContext) uint32 {
	// Hash packet for flow table lookup
	ipv4, ipv6, _ := pkt.ParseAllKnownL3()
	if ipv4 != nil {
		return types.HashCombine(uint32(ipv4.SrcAddr), uint32(ipv4.DstAddr))
	} else if ipv6 != nil {
		return types.HashCombine(
			*(*uint32)(unsafe.Pointer(&ipv6.SrcAddr[0])),
			*(*uint32)(unsafe.Pointer(&ipv6.DstAddr[0])),
		)
	}
	return 0
}

// collectStats collects and logs network statistics
func (m *DPDKManager) collectStats(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.done:
			return
		case <-ticker.C:
			m.logger.Info("Network statistics",
				zap.Uint64("rx_packets", m.stats.rxPackets.Load()),
				zap.Uint64("tx_packets", m.stats.txPackets.Load()),
				zap.Uint64("rx_bytes", m.stats.rxBytes.Load()),
				zap.Uint64("tx_bytes", m.stats.txBytes.Load()),
				zap.Uint64("rx_drops", m.stats.rxDrops.Load()),
				zap.Uint64("tx_drops", m.stats.txDrops.Load()),
				zap.Duration("latency", time.Duration(m.stats.latencyNs.Load())),
			)
		}
	}
}

// GetStats returns current network statistics
func (m *DPDKManager) GetStats() (rxPackets, txPackets, rxBytes, txBytes, rxDrops, txDrops uint64, latencyNs uint64) {
	return m.stats.rxPackets.Load(),
		m.stats.txPackets.Load(),
		m.stats.rxBytes.Load(),
		m.stats.txBytes.Load(),
		m.stats.rxDrops.Load(),
		m.stats.txDrops.Load(),
		m.stats.latencyNs.Load()
}
