//go:build dpdk
// +build dpdk

// Package network provides network optimization utilities for the MEV bot
package network

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

// NetworkOptimizer represents a high-performance network interface
type NetworkOptimizer struct {
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *zap.Logger
	config  *Config
	mu      sync.RWMutex
	metrics struct {
		sendLatency    prometheus.Histogram
		receiveLatency prometheus.Histogram
		bytesIn        prometheus.Counter
		bytesOut       prometheus.Counter
		errors         prometheus.Counter
		reconnects     prometheus.Counter
		connUptime     prometheus.Gauge
		queueDepth     prometheus.Gauge
		cpuUsage       prometheus.Gauge
	}
	dpdkEal   *dpdk.EalConfig
	dpdkPorts []uint16
	hugepages []string
	isClosed  bool
}

// Config holds the network optimizer configuration
type Config struct {
	DialTimeout      time.Duration
	KeepAlivePeriod  time.Duration
	ReconnectBackoff time.Duration
	MaxReconnects    int
	MetricsInterval  time.Duration
	ReadBufferSize   int
	WriteBufferSize  int
	CPUAffinity      []int // CPU cores to pin to
	RxQueueSize      int   // Receive queue size
	TxQueueSize      int   // Transmit queue size
	HugepageDir      string
	NumHugepages     int
	DPDKConfig       struct {
		PortMask    uint64
		NumRxQueues uint16
		NumTxQueues uint16
		NumMbufs    uint32
		MbufSize    uint32
		BurstSize   uint16
	}
	System struct {
		CPUPinning      uint64
		DPDKMemChannels int
		MemoryLimit     int
		HugePageSize    int
		HugePagesCount  int
		DPDKPorts       []uint16
	}
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	cfg := &Config{
		DialTimeout:      5 * time.Second,
		KeepAlivePeriod:  30 * time.Second,
		ReconnectBackoff: 1 * time.Second,
		MaxReconnects:    5,
		MetricsInterval:  30 * time.Second,
		ReadBufferSize:   32 * 1024, // 32KB
		WriteBufferSize:  32 * 1024, // 32KB
		CPUAffinity:      []int{0},  // Default to first CPU
		RxQueueSize:      4096,
		TxQueueSize:      4096,
		HugepageDir:      "/dev/hugepages",
		NumHugepages:     1024,
	}

	cfg.DPDKConfig.PortMask = 0x1 // Use first port
	cfg.DPDKConfig.NumRxQueues = 1
	cfg.DPDKConfig.NumTxQueues = 1
	cfg.DPDKConfig.NumMbufs = 8192
	cfg.DPDKConfig.MbufSize = 2048
	cfg.DPDKConfig.BurstSize = 32

	cfg.System.CPUPinning = 1
	cfg.System.DPDKMemChannels = 4
	cfg.System.MemoryLimit = 1024
	cfg.System.HugePageSize = 2048
	cfg.System.HugePagesCount = 1024
	cfg.System.DPDKPorts = []uint16{0}

	return cfg
}

// NewNetworkOptimizer creates a new network optimizer instance
func NewNetworkOptimizer(ctx context.Context, logger *zap.Logger, config *Config) (*NetworkOptimizer, error) {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(ctx)

	opt := &NetworkOptimizer{
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
		config: config,
	}

	// Initialize metrics
	opt.metrics.sendLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "network_send_latency_seconds",
		Help:    "Histogram of network send latencies in seconds",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10), // Start at 0.1ms, double 10 times
	})
	opt.metrics.receiveLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "network_receive_latency_seconds",
		Help:    "Histogram of network receive latencies in seconds",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10),
	})
	opt.metrics.bytesIn = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "network_bytes_received_total",
		Help: "Total number of bytes received",
	})
	opt.metrics.bytesOut = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "network_bytes_sent_total",
		Help: "Total number of bytes sent",
	})
	opt.metrics.errors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "network_errors_total",
		Help: "Total number of network errors",
	})
	opt.metrics.reconnects = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "network_reconnects_total",
		Help: "Total number of network reconnections",
	})
	opt.metrics.connUptime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_connection_uptime_seconds",
		Help: "Current connection uptime in seconds",
	})
	opt.metrics.queueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_queue_depth",
		Help: "Current network queue depth",
	})
	opt.metrics.cpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_cpu_usage_percent",
		Help: "Current CPU usage percentage",
	})

	// Register metrics
	prometheus.MustRegister(opt.metrics.sendLatency)
	prometheus.MustRegister(opt.metrics.receiveLatency)
	prometheus.MustRegister(opt.metrics.bytesIn)
	prometheus.MustRegister(opt.metrics.bytesOut)
	prometheus.MustRegister(opt.metrics.errors)
	prometheus.MustRegister(opt.metrics.reconnects)
	prometheus.MustRegister(opt.metrics.connUptime)
	prometheus.MustRegister(opt.metrics.queueDepth)
	prometheus.MustRegister(opt.metrics.cpuUsage)

	// Initialize DPDK
	if err := opt.initDPDK(); err != nil {
		return nil, err
	}

	// Set CPU affinity
	if err := opt.setCPUAffinity(); err != nil {
		logger.Warn("failed to set CPU affinity", zap.Error(err))
	}

	// Start metrics reporting
	go opt.reportMetrics()

	return opt, nil
}

func (n *NetworkOptimizer) initDPDK() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.isClosed {
		return errors.New("network optimizer is closed")
	}

	// Initialize EAL configuration
	n.dpdkEal = &dpdk.EalConfig{
		CoreMask:       n.config.System.CPUPinning,
		MemoryChannels: n.config.System.DPDKMemChannels,
		MemorySize:     n.config.System.MemoryLimit,
	}

	// Initialize DPDK environment
	if err := dpdk.EalInit(n.dpdkEal); err != nil {
		n.logger.Error("Failed to initialize DPDK EAL",
			zap.Error(err),
			zap.Any("config", n.dpdkEal))
		return fmt.Errorf("DPDK EAL initialization failed: %w", err)
	}

	// Configure huge pages
	if err := n.setupHugePages(); err != nil {
		n.logger.Error("Failed to setup huge pages",
			zap.Error(err),
			zap.String("size", fmt.Sprintf("%dMB", n.config.System.HugePageSize)),
			zap.Int("count", n.config.System.HugePagesCount))
		return fmt.Errorf("huge pages setup failed: %w", err)
	}

	// Initialize DPDK ports
	for _, portID := range n.config.System.DPDKPorts {
		if err := n.initDPDKPort(portID); err != nil {
			n.logger.Error("Failed to initialize DPDK port",
				zap.Error(err),
				zap.Uint16("port", portID))
			return fmt.Errorf("DPDK port initialization failed: %w", err)
		}
		n.dpdkPorts = append(n.dpdkPorts, portID)
	}

	// Optimize queues
	for _, portID := range n.dpdkPorts {
		if err := n.OptimizeRxQueue(portID); err != nil {
			return fmt.Errorf("failed to optimize RX queue: %w", err)
		}
		if err := n.OptimizeTxQueue(portID); err != nil {
			return fmt.Errorf("failed to optimize TX queue: %w", err)
		}
	}

	// Start metrics collection
	go n.collectPortMetrics()

	n.logger.Info("DPDK initialization completed successfully",
		zap.Uint16s("ports", n.dpdkPorts),
		zap.Int("memory_channels", n.config.System.DPDKMemChannels))

	return nil
}

func (n *NetworkOptimizer) initDPDKPort(portID uint16) error {
	// Configure port
	config := dpdk.PortConfig{
		RxQueues: uint16(runtime.GOMAXPROCS(-1)),
		TxQueues: uint16(runtime.GOMAXPROCS(-1)),
		RxDesc:   1024,
		TxDesc:   1024,
	}

	if err := dpdk.PortConfigure(portID, &config); err != nil {
		return fmt.Errorf("port configuration failed: %w", err)
	}

	// Start port
	if err := dpdk.PortStart(portID); err != nil {
		return fmt.Errorf("port start failed: %w", err)
	}

	return nil
}

func (n *NetworkOptimizer) OptimizeRxQueue(portID uint16) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Configure RX queue with optimal parameters
	rxConf := dpdk.RxQueueConfig{
		NumDescriptors: 4096,
		SocketID:      dpdk.SOCKET_ID_ANY,
		QueueID:       0,
	}

	// Enable scatter-gather and checksum offload
	rxConf.EnableRSS = true
	rxConf.EnableLRO = true
	rxConf.EnableChecksumOffload = true

	return dpdk.SetupRxQueue(portID, &rxConf)
}

func (n *NetworkOptimizer) OptimizeTxQueue(portID uint16) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Configure TX queue with optimal parameters
	txConf := dpdk.TxQueueConfig{
		NumDescriptors: 4096,
		SocketID:      dpdk.SOCKET_ID_ANY,
		QueueID:       0,
	}

	// Enable TSO and checksum offload
	txConf.EnableTSO = true
	txConf.EnableChecksumOffload = true

	return dpdk.SetupTxQueue(portID, &txConf)
}

func (n *NetworkOptimizer) collectPortMetrics() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			for _, portID := range n.dpdkPorts {
				stats, err := dpdk.GetPortStats(portID)
				if err != nil {
					n.logger.Error("Failed to get port stats",
						zap.Error(err),
						zap.Uint16("port", portID))
					continue
				}

				n.metrics.bytesIn.Add(float64(stats.RxBytes))
				n.metrics.bytesOut.Add(float64(stats.TxBytes))
				n.metrics.queueDepth.Set(float64(stats.RxQueueSize))
			}
		}
	}
}

func (n *NetworkOptimizer) setCPUAffinity() error {
	if len(n.config.CPUAffinity) == 0 {
		return nil
	}

	// Lock OS thread to maintain CPU affinity
	runtime.LockOSThread()

	// Set CPU affinity using netlink
	cpuset := &netlink.CPUSet{}
	for _, cpu := range n.config.CPUAffinity {
		cpuset.Set(cpu)
	}

	if err := syscall.SchedSetaffinity(0, cpuset); err != nil {
		return fmt.Errorf("failed to set CPU affinity: %w", err)
	}

	return nil
}

func (n *NetworkOptimizer) setupHugePages() error {
	// Create hugepage directory if it doesn't exist
	if err := os.MkdirAll(n.config.HugepageDir, 0755); err != nil {
		return fmt.Errorf("failed to create hugepage directory: %w", err)
	}

	// Mount hugetlbfs if not already mounted
	if err := syscall.Mount("none", n.config.HugepageDir, "hugetlbfs", 0, ""); err != nil {
		if !errors.Is(err, syscall.EBUSY) { // Ignore if already mounted
			return fmt.Errorf("failed to mount hugetlbfs: %w", err)
		}
	}

	// Allocate huge pages
	for i := 0; i < n.config.System.HugePagesCount; i++ {
		path := filepath.Join(n.config.HugepageDir, fmt.Sprintf("page%d", i))
		fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR, 0644)
		if err != nil {
			return fmt.Errorf("failed to create hugepage file: %w", err)
		}
		syscall.Close(fd)
		n.hugepages = append(n.hugepages, path)
	}

	return nil
}

// Cleanup releases all resources
func (n *NetworkOptimizer) Cleanup() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.isClosed {
		return nil
	}

	// Stop DPDK
	dpdk.EalCleanup()

	// Cleanup huge pages
	for _, page := range n.hugepages {
		os.Remove(page)
	}

	// Unmount hugetlbfs
	syscall.Unmount(n.config.HugepageDir, 0)

	n.isClosed = true
	n.cancel()
	return nil
}

// reportMetrics periodically logs performance metrics
func (n *NetworkOptimizer) reportMetrics() {
	ticker := time.NewTicker(n.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.mu.RLock()
			if n.isClosed {
				n.mu.RUnlock()
				return
			}

			n.metrics.connUptime.Set(time.Since(time.Now()).Seconds())
			n.logger.Info("network metrics",
				zap.Float64("send_latency_ms", n.metrics.sendLatency.Sum()/(float64(time.Millisecond))),
				zap.Float64("receive_latency_ms", n.metrics.receiveLatency.Sum()/(float64(time.Millisecond))),
				zap.Float64("bytes_in", n.metrics.bytesIn.Value()),
				zap.Float64("bytes_out", n.metrics.bytesOut.Value()),
				zap.Float64("errors", n.metrics.errors.Value()),
				zap.Float64("reconnects", n.metrics.reconnects.Value()),
				zap.Float64("uptime_hours", n.metrics.connUptime.Value()))
			n.mu.RUnlock()
		}
	}
}
