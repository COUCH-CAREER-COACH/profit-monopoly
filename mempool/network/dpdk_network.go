//go:build dpdk
// +build dpdk

package network

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// #cgo CFLAGS: -m64 -pthread -O3 -march=native -I/usr/local/include/dpdk
// #cgo LDFLAGS: -L/usr/local/lib -ldpdk
// #include <rte_config.h>
// #include <rte_eal.h>
// #include <rte_ethdev.h>
// #include <rte_mbuf.h>
// #include <rte_mempool.h>
// #include <rte_ring.h>
import "C"

const (
	numMbufs      = 8191
	mbufCacheSize = 250
	rxRingSize    = 1024
	txRingSize    = 1024
	maxRxPktBurst = 32
	maxTxPktBurst = 32
	numRxQueues   = 1
	numTxQueues   = 1
	hugepageSize  = 1 << 21 // 2MB hugepages
)

// DPDKNetworkManager implements NetworkManager using DPDK
type DPDKNetworkManager struct {
	logger  *zap.Logger
	config  *NetworkConfig
	mempool *C.struct_rte_mempool
	portID  uint16
	rxRings []*C.struct_rte_ring
	txRings []*C.struct_rte_ring
	running atomic.Bool
	wg      sync.WaitGroup
	metrics struct {
		rxPackets prometheus.Counter
		txPackets prometheus.Counter
		rxBytes   prometheus.Counter
		txBytes   prometheus.Counter
		rxErrors  prometheus.Counter
		txErrors  prometheus.Counter
	}
}

// NewDPDKNetworkManager creates a new DPDK-based network manager
func NewDPDKNetworkManager(logger *zap.Logger, config *NetworkConfig) (NetworkManager, error) {
	if err := initializeDPDK(); err != nil {
		return nil, fmt.Errorf("failed to initialize DPDK: %v", err)
	}

	mgr := &DPDKNetworkManager{
		logger: logger,
		config: config,
	}

	// Initialize metrics
	mgr.metrics.rxPackets = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dpdk_rx_packets_total",
		Help: "Total number of packets received",
	})
	mgr.metrics.txPackets = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dpdk_tx_packets_total",
		Help: "Total number of packets transmitted",
	})
	mgr.metrics.rxBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dpdk_rx_bytes_total",
		Help: "Total number of bytes received",
	})
	mgr.metrics.txBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dpdk_tx_bytes_total",
		Help: "Total number of bytes transmitted",
	})
	mgr.metrics.rxErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dpdk_rx_errors_total",
		Help: "Total number of receive errors",
	})
	mgr.metrics.txErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dpdk_tx_errors_total",
		Help: "Total number of transmit errors",
	})

	// Create mempool for packet buffers
	mempoolName := C.CString("mbuf_pool")
	defer C.free(unsafe.Pointer(mempoolName))

	mgr.mempool = C.rte_pktmbuf_pool_create(mempoolName, numMbufs,
		mbufCacheSize, 0, C.RTE_MBUF_DEFAULT_BUF_SIZE,
		C.rte_socket_id())

	if mgr.mempool == nil {
		return nil, fmt.Errorf("failed to create mempool")
	}

	// Initialize port
	if err := mgr.initializePort(); err != nil {
		return nil, err
	}

	// Create rings
	if err := mgr.createRings(); err != nil {
		return nil, err
	}

	// Pin CPU cores
	if err := mgr.pinCPUCores(); err != nil {
		logger.Warn("Failed to pin CPU cores", zap.Error(err))
	}

	return mgr, nil
}

// initializeDPDK initializes the DPDK EAL
func initializeDPDK() error {
	args := []string{
		"mev-bot",
		"-l", "0-3", // Use cores 0-3
		"-n", "4", // 4 memory channels
		"--huge-dir", "/mnt/huge",
		"--proc-type=auto",
	}

	cArgs := make([]*C.char, len(args))
	for i, arg := range args {
		cArgs[i] = C.CString(arg)
		defer C.free(unsafe.Pointer(cArgs[i]))
	}

	argc := C.int(len(args))
	argv := (**C.char)(unsafe.Pointer(&cArgs[0]))

	if ret := C.rte_eal_init(argc, argv); ret < 0 {
		return fmt.Errorf("failed to initialize EAL")
	}

	return nil
}

// initializePort configures the network port
func (m *DPDKNetworkManager) initializePort() error {
	// Get the first available port
	m.portID = 0

	// Configure the Ethernet device
	var portConf C.struct_rte_eth_conf
	portConf.rxmode.max_rx_pkt_len = C.RTE_ETHER_MAX_LEN

	if ret := C.rte_eth_dev_configure(m.portID, numRxQueues, numTxQueues, &portConf); ret < 0 {
		return fmt.Errorf("failed to configure device: %d", ret)
	}

	// Set up RX queue
	if ret := C.rte_eth_rx_queue_setup(m.portID, 0, rxRingSize,
		C.rte_eth_dev_socket_id(m.portID), nil, m.mempool); ret < 0 {
		return fmt.Errorf("failed to setup rx queue: %d", ret)
	}

	// Set up TX queue
	if ret := C.rte_eth_tx_queue_setup(m.portID, 0, txRingSize,
		C.rte_eth_dev_socket_id(m.portID), nil); ret < 0 {
		return fmt.Errorf("failed to setup tx queue: %d", ret)
	}

	// Start the Ethernet port
	if ret := C.rte_eth_dev_start(m.portID); ret < 0 {
		return fmt.Errorf("failed to start device: %d", ret)
	}

	return nil
}

// createRings creates the rx and tx rings
func (m *DPDKNetworkManager) createRings() error {
	m.rxRings = make([]*C.struct_rte_ring, numRxQueues)
	m.txRings = make([]*C.struct_rte_ring, numTxQueues)

	for i := 0; i < numRxQueues; i++ {
		ringName := C.CString(fmt.Sprintf("rx_ring_%d", i))
		defer C.free(unsafe.Pointer(ringName))

		m.rxRings[i] = C.rte_ring_create(ringName, rxRingSize, C.rte_socket_id(),
			C.RING_F_SP_ENQ|C.RING_F_SC_DEQ)
		if m.rxRings[i] == nil {
			return fmt.Errorf("failed to create rx ring %d", i)
		}
	}

	for i := 0; i < numTxQueues; i++ {
		ringName := C.CString(fmt.Sprintf("tx_ring_%d", i))
		defer C.free(unsafe.Pointer(ringName))

		m.txRings[i] = C.rte_ring_create(ringName, txRingSize, C.rte_socket_id(),
			C.RING_F_SP_ENQ|C.RING_F_SC_DEQ)
		if m.txRings[i] == nil {
			return fmt.Errorf("failed to create tx ring %d", i)
		}
	}

	return nil
}

// pinCPUCores pins critical threads to specific CPU cores
func (m *DPDKNetworkManager) pinCPUCores() error {
	if len(m.config.CPUAffinity) == 0 {
		return nil
	}

	// Create CPU set
	cpuset := &syscall.CPUSet{}
	for _, cpu := range m.config.CPUAffinity {
		cpuset.Set(cpu)
	}

	// Pin the current thread
	runtime.LockOSThread()
	if err := syscall.SchedSetaffinity(0, cpuset); err != nil {
		return fmt.Errorf("failed to set CPU affinity: %v", err)
	}

	return nil
}

// Start starts the network manager
func (m *DPDKNetworkManager) Start(ctx context.Context) error {
	if !m.running.CompareAndSwap(false, true) {
		return fmt.Errorf("network manager already running")
	}

	m.wg.Add(2)
	go m.rxLoop(ctx)
	go m.txLoop(ctx)

	return nil
}

// Stop stops the network manager
func (m *DPDKNetworkManager) Stop() error {
	if !m.running.CompareAndSwap(true, false) {
		return nil
	}

	m.wg.Wait()
	C.rte_eth_dev_stop(m.portID)
	C.rte_eth_dev_close(m.portID)

	return nil
}

// rxLoop handles packet reception
func (m *DPDKNetworkManager) rxLoop(ctx context.Context) {
	defer m.wg.Done()

	bufs := make([]*C.struct_rte_mbuf, maxRxPktBurst)
	for m.running.Load() {
		select {
		case <-ctx.Done():
			return
		default:
			// Receive packets
			nb_rx := C.rte_eth_rx_burst(m.portID, 0, (**C.struct_rte_mbuf)(unsafe.Pointer(&bufs[0])), maxRxPktBurst)
			if nb_rx > 0 {
				m.metrics.rxPackets.Add(float64(nb_rx))

				// Process received packets
				for i := 0; i < int(nb_rx); i++ {
					pkt := bufs[i]
					m.metrics.rxBytes.Add(float64(pkt.data_len))

					// Enqueue to rx ring
					if C.rte_ring_enqueue(m.rxRings[0], unsafe.Pointer(pkt)) < 0 {
						m.metrics.rxErrors.Inc()
						C.rte_pktmbuf_free(pkt)
					}
				}
			}
		}
	}
}

// txLoop handles packet transmission
func (m *DPDKNetworkManager) txLoop(ctx context.Context) {
	defer m.wg.Done()

	bufs := make([]*C.struct_rte_mbuf, maxTxPktBurst)
	for m.running.Load() {
		select {
		case <-ctx.Done():
			return
		default:
			// Dequeue packets from tx ring
			nb_tx := C.rte_ring_dequeue_burst(m.txRings[0],
				(*unsafe.Pointer)(unsafe.Pointer(&bufs[0])),
				maxTxPktBurst, nil)

			if nb_tx > 0 {
				// Transmit packets
				sent := C.rte_eth_tx_burst(m.portID, 0,
					(**C.struct_rte_mbuf)(unsafe.Pointer(&bufs[0])), nb_tx)

				m.metrics.txPackets.Add(float64(sent))

				// Free unsent packets
				if sent < nb_tx {
					m.metrics.txErrors.Add(float64(nb_tx - sent))
					for i := sent; i < nb_tx; i++ {
						C.rte_pktmbuf_free(bufs[i])
					}
				}
			}
		}
	}
}

// Read reads data from the network
func (m *DPDKNetworkManager) Read(p []byte) (n int, err error) {
	var pkt *C.struct_rte_mbuf
	if C.rte_ring_dequeue(m.rxRings[0], (*unsafe.Pointer)(unsafe.Pointer(&pkt))) < 0 {
		return 0, nil
	}

	data := C.rte_pktmbuf_mtod(pkt, *C.char)
	length := int(pkt.data_len)
	if length > len(p) {
		length = len(p)
	}

	copy(p, unsafe.Slice((*byte)(unsafe.Pointer(data)), length))
	C.rte_pktmbuf_free(pkt)

	return length, nil
}

// Write writes data to the network
func (m *DPDKNetworkManager) Write(p []byte) (n int, err error) {
	pkt := C.rte_pktmbuf_alloc(m.mempool)
	if pkt == nil {
		return 0, fmt.Errorf("failed to allocate mbuf")
	}

	data := C.rte_pktmbuf_mtod(pkt, *C.char)
	length := len(p)

	// Copy data to mbuf
	C.rte_memcpy(unsafe.Pointer(data), unsafe.Pointer(&p[0]), C.size_t(length))
	pkt.data_len = C.uint16_t(length)
	pkt.pkt_len = C.uint32_t(length)

	// Enqueue packet for transmission
	if C.rte_ring_enqueue(m.txRings[0], unsafe.Pointer(pkt)) < 0 {
		C.rte_pktmbuf_free(pkt)
		return 0, fmt.Errorf("failed to enqueue packet")
	}

	return length, nil
}
