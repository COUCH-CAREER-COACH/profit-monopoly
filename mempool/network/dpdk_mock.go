//go:build !dpdk
// +build !dpdk

package network

import (
	"context"
	"errors"
	"net"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/michaelpento.lv/mevbot/utils/metrics"
)

// MockDPDKManager implements NetworkManager interface for testing
type MockDPDKManager struct{}

func NewDPDKNetworkManager(cfg *NetworkConfig) (NetworkManager, error) {
	return &MockDPDKManager{}, nil
}

func (m *MockDPDKManager) Initialize(ctx context.Context) error {
	return nil
}

func (m *MockDPDKManager) Start() error {
	return nil
}

func (m *MockDPDKManager) Stop() error {
	return nil
}

func (m *MockDPDKManager) SendTransaction(tx []byte) error {
	return nil
}

func (m *MockDPDKManager) ReceiveTransactions() ([][]byte, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *MockDPDKManager) ProcessTransaction(tx *types.Transaction) error {
	return nil
}

func (m *MockDPDKManager) Read(p []byte) (n int, err error) {
	return 0, errors.New("not implemented in mock")
}

func (m *MockDPDKManager) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (m *MockDPDKManager) ReadPacket() ([]byte, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *MockDPDKManager) WritePacket(p []byte) error {
	return nil
}

func (m *MockDPDKManager) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"packets_received": 0,
		"packets_sent":     0,
		"errors":           0,
	}
}

// Queue optimization methods
func (m *MockDPDKManager) OptimizeRxQueue() (*QueueStats, error) {
	return &QueueStats{}, nil
}

func (m *MockDPDKManager) OptimizeTxQueue() (*QueueStats, error) {
	return &QueueStats{}, nil
}

func (m *MockDPDKManager) GetPortMetrics() *metrics.NetworkMetrics {
	return metrics.NewNetworkMetrics("mock.dpdk")
}

// Connection optimization methods
func (m *MockDPDKManager) OptimizeConn(conn net.Conn) error {
	return nil
}

func (m *MockDPDKManager) Send(conn net.Conn, data []byte) error {
	return nil
}

func (m *MockDPDKManager) Receive(conn net.Conn, buffer []byte) (int, error) {
	return 0, nil
}

// Huge pages management
func (m *MockDPDKManager) SetupHugePages() error {
	return nil
}

func (m *MockDPDKManager) AreHugePagesEnabled() bool {
	return false
}
