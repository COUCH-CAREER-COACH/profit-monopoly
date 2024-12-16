package mempool

import (
	"sync"
	"testing"
)

// MockBPFMonitor implements a mock version of our eBPF monitoring
type MockBPFMonitor struct {
	mu sync.Mutex
	metrics map[string]uint64
}

func NewMockBPFMonitor() *MockBPFMonitor {
	return &MockBPFMonitor{
		metrics: make(map[string]uint64),
	}
}

func (m *MockBPFMonitor) Start() error {
	return nil
}

func (m *MockBPFMonitor) Stop() error {
	return nil
}

func (m *MockBPFMonitor) GetMetrics() map[string]uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	result := make(map[string]uint64)
	for k, v := range m.metrics {
		result[k] = v
	}
	return result
}

// MockDPDKManager implements a mock version of our DPDK network manager
type MockDPDKManager struct {
	mu sync.Mutex
	packets [][]byte
}

func NewMockDPDKManager() *MockDPDKManager {
	return &MockDPDKManager{
		packets: make([][]byte, 0),
	}
}

func (m *MockDPDKManager) Start() error {
	return nil
}

func (m *MockDPDKManager) Stop() error {
	return nil
}

func (m *MockDPDKManager) SendPacket(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	packetCopy := make([]byte, len(data))
	copy(packetCopy, data)
	m.packets = append(m.packets, packetCopy)
	return nil
}

func TestMockBPFMonitor(t *testing.T) {
	monitor := NewMockBPFMonitor()
	
	err := monitor.Start()
	if err != nil {
		t.Fatalf("Failed to start mock BPF monitor: %v", err)
	}
	
	metrics := monitor.GetMetrics()
	if len(metrics) != 0 {
		t.Errorf("Expected empty metrics, got %d entries", len(metrics))
	}
	
	err = monitor.Stop()
	if err != nil {
		t.Fatalf("Failed to stop mock BPF monitor: %v", err)
	}
}

func TestMockDPDKManager(t *testing.T) {
	manager := NewMockDPDKManager()
	
	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start mock DPDK manager: %v", err)
	}
	
	testData := []byte("test packet")
	err = manager.SendPacket(testData)
	if err != nil {
		t.Fatalf("Failed to send test packet: %v", err)
	}
	
	if len(manager.packets) != 1 {
		t.Errorf("Expected 1 packet, got %d packets", len(manager.packets))
	}
	
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop mock DPDK manager: %v", err)
	}
}
