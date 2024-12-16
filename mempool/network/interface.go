package network

import (
	"context"
	"io"
	"net"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/michaelpento.lv/mevbot/utils/metrics"
)

// NetworkReader defines interface for reading from network
type NetworkReader interface {
	io.Reader
	ReadPacket() ([]byte, error)
}

// NetworkWriter defines interface for writing to network
type NetworkWriter interface {
	io.Writer
	WritePacket([]byte) error
}

// NetworkIO combines reader and writer interfaces
type NetworkIO interface {
	NetworkReader
	NetworkWriter
}

// NetworkManager defines the interface for network optimizations
type NetworkManager interface {
	NetworkIO
	Initialize(ctx context.Context) error
	Start() error
	Stop() error
	ProcessTransaction(tx *types.Transaction) error
	GetMetrics() map[string]interface{}
	
	// Queue optimization methods
	OptimizeRxQueue() (*QueueStats, error)
	OptimizeTxQueue() (*QueueStats, error)
	GetPortMetrics() *metrics.NetworkMetrics
	
	// Connection optimization methods
	OptimizeConn(conn net.Conn) error
	Send(conn net.Conn, data []byte) error
	Receive(conn net.Conn, buffer []byte) (int, error)
	
	// Huge pages management
	SetupHugePages() error
	AreHugePagesEnabled() bool
}
