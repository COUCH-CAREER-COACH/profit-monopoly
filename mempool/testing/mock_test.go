package testing

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

func (m *MockBPFMonitor) RecordSyscall(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics[name]++
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

func TestMockBPFMonitor(t *testing.T) {
	monitor := NewMockBPFMonitor()
	
	// Record some syscalls
	monitor.RecordSyscall("read")
	monitor.RecordSyscall("write")
	monitor.RecordSyscall("read")
	
	metrics := monitor.GetMetrics()
	if metrics["read"] != 2 {
		t.Errorf("Expected 2 read syscalls, got %d", metrics["read"])
	}
	if metrics["write"] != 1 {
		t.Errorf("Expected 1 write syscall, got %d", metrics["write"])
	}
}

// MockTransaction represents a simplified transaction for testing
type MockTransaction struct {
	Hash string
	Data []byte
}

// MockMempoolMonitor implements a mock version of our mempool monitoring
type MockMempoolMonitor struct {
	mu sync.Mutex
	txs map[string]*MockTransaction
}

func NewMockMempoolMonitor() *MockMempoolMonitor {
	return &MockMempoolMonitor{
		txs: make(map[string]*MockTransaction),
	}
}

func (m *MockMempoolMonitor) AddTransaction(tx *MockTransaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs[tx.Hash] = tx
	return nil
}

func (m *MockMempoolMonitor) GetTransaction(hash string) (*MockTransaction, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tx, exists := m.txs[hash]
	return tx, exists
}

func TestMockMempoolMonitor(t *testing.T) {
	monitor := NewMockMempoolMonitor()
	
	// Add a transaction
	tx := &MockTransaction{
		Hash: "0x123",
		Data: []byte("test transaction"),
	}
	
	err := monitor.AddTransaction(tx)
	if err != nil {
		t.Fatalf("Failed to add transaction: %v", err)
	}
	
	// Retrieve the transaction
	retrieved, exists := monitor.GetTransaction(tx.Hash)
	if !exists {
		t.Fatal("Transaction not found")
	}
	
	if retrieved.Hash != tx.Hash {
		t.Errorf("Expected hash %s, got %s", tx.Hash, retrieved.Hash)
	}
	
	// Try to get non-existent transaction
	_, exists = monitor.GetTransaction("0xnonexistent")
	if exists {
		t.Error("Found non-existent transaction")
	}
}
