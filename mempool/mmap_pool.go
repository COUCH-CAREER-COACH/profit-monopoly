package mempool

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/sys/unix"
)

const (
	// Size constants
	pageSize        = 4096
	txEntrySize     = 512 // Fixed size for each transaction entry
	maxTransactions = 100000
	
	// Memory map flags
	mmapProt  = syscall.PROT_READ | syscall.PROT_WRITE
	mmapFlags = syscall.MAP_SHARED | syscall.MAP_HUGETLB | (30 << unix.MAP_HUGE_SHIFT) // Use 1GB huge pages
)

// MMapPool implements a memory-mapped transaction pool
type MMapPool struct {
	file     *os.File
	data     []byte
	metadata *metadata
	mu       sync.RWMutex
	indices  map[common.Hash]uint32 // Hash -> index mapping
}

// metadata is stored at the start of the memory-mapped file
type metadata struct {
	Count    uint32
	NextFree uint32
}

// NewMMapPool creates a new memory-mapped transaction pool
func NewMMapPool(filepath string) (*MMapPool, error) {
	// Create or open the file
	file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Set file size to accommodate metadata and transactions
	size := int64(unsafe.Sizeof(metadata{})) + int64(maxTransactions)*int64(txEntrySize)
	if err := file.Truncate(size); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to truncate file: %w", err)
	}

	// Memory map the file with huge pages
	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), mmapProt, mmapFlags)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to mmap: %w", err)
	}

	pool := &MMapPool{
		file:    file,
		data:    data,
		metadata: (*metadata)(unsafe.Pointer(&data[0])),
		indices: make(map[common.Hash]uint32),
	}

	// Lock the pages in memory
	if err := unix.Mlock(data); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to mlock: %w", err)
	}

	return pool, nil
}

// Add adds a transaction to the pool
func (p *MMapPool) Add(tx *types.Transaction) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.metadata.Count >= maxTransactions {
		return fmt.Errorf("pool is full")
	}

	// Calculate offset for new transaction
	offset := int(unsafe.Sizeof(*p.metadata)) + int(p.metadata.NextFree)*txEntrySize

	// Serialize transaction
	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	if len(txBytes) > txEntrySize {
		return fmt.Errorf("transaction too large")
	}

	// Copy transaction data to memory-mapped file
	copy(p.data[offset:offset+len(txBytes)], txBytes)

	// Update metadata
	p.indices[tx.Hash()] = p.metadata.NextFree
	p.metadata.Count++
	p.metadata.NextFree = p.findNextFree()

	return nil
}

// Get retrieves a transaction by its hash
func (p *MMapPool) Get(hash common.Hash) (*types.Transaction, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	index, exists := p.indices[hash]
	if !exists {
		return nil, false
	}

	offset := int(unsafe.Sizeof(*p.metadata)) + int(index)*txEntrySize
	
	// Read transaction data
	var tx types.Transaction
	if err := tx.UnmarshalBinary(p.data[offset : offset+txEntrySize]); err != nil {
		return nil, false
	}

	return &tx, true
}

// Remove removes a transaction from the pool
func (p *MMapPool) Remove(hash common.Hash) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	index, exists := p.indices[hash]
	if !exists {
		return false
	}

	delete(p.indices, hash)
	p.metadata.Count--

	// Mark slot as free by zeroing it out
	offset := int(unsafe.Sizeof(*p.metadata)) + int(index)*txEntrySize
	for i := 0; i < txEntrySize; i++ {
		p.data[offset+i] = 0
	}

	return true
}

// findNextFree finds the next free slot in the pool
func (p *MMapPool) findNextFree() uint32 {
	dataStart := int(unsafe.Sizeof(*p.metadata))
	for i := uint32(0); i < maxTransactions; i++ {
		offset := dataStart + int(i)*txEntrySize
		isEmpty := true
		for j := 0; j < txEntrySize; j++ {
			if p.data[offset+j] != 0 {
				isEmpty = false
				break
			}
		}
		if isEmpty {
			return i
		}
	}
	return 0 // Pool is full
}

// Close closes the memory-mapped file
func (p *MMapPool) Close() error {
	if err := unix.Munlock(p.data); err != nil {
		return fmt.Errorf("failed to munlock: %w", err)
	}

	if err := syscall.Munmap(p.data); err != nil {
		return fmt.Errorf("failed to munmap: %w", err)
	}

	return p.file.Close()
}

// Stats returns pool statistics
func (p *MMapPool) Stats() (count uint32, capacity uint32) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata.Count, maxTransactions
}
