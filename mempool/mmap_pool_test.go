package mempool

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestMMapPool(t *testing.T) {
	// Create temporary file for testing
	tmpDir, err := os.MkdirTemp("", "mmap_pool_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	poolPath := filepath.Join(tmpDir, "test.mmap")

	t.Run("Basic Operations", func(t *testing.T) {
		// Create pool
		pool, err := NewMMapPool(poolPath)
		require.NoError(t, err)
		defer pool.Close()

		// Create test transaction
		tx := createTestTransaction(t)
		
		// Test Add
		err = pool.Add(tx)
		require.NoError(t, err)

		// Test Get
		retrievedTx, exists := pool.Get(tx.Hash())
		require.True(t, exists)
		require.Equal(t, tx.Hash(), retrievedTx.Hash())

		// Test Remove
		removed := pool.Remove(tx.Hash())
		require.True(t, removed)

		// Verify removal
		_, exists = pool.Get(tx.Hash())
		require.False(t, exists)
	})

	t.Run("Capacity Limits", func(t *testing.T) {
		pool, err := NewMMapPool(poolPath)
		require.NoError(t, err)
		defer pool.Close()

		// Add transactions until full
		for i := 0; i < maxTransactions+1; i++ {
			tx := createTestTransaction(t)
			err := pool.Add(tx)
			if i < maxTransactions {
				require.NoError(t, err)
			} else {
				require.Error(t, err) // Should fail when pool is full
			}
		}
	})

	t.Run("Concurrent Operations", func(t *testing.T) {
		pool, err := NewMMapPool(poolPath)
		require.NoError(t, err)
		defer pool.Close()

		// Create workers
		numWorkers := 10
		numOps := 100
		var wg sync.WaitGroup
		errors := make(chan error, numWorkers*numOps)

		// Start workers
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < numOps; j++ {
					// Random operation: Add, Get, or Remove
					switch j % 3 {
					case 0:
						tx := createTestTransaction(t)
						if err := pool.Add(tx); err != nil {
							errors <- fmt.Errorf("worker %d: add error: %v", workerID, err)
						}
					case 1:
						tx := createTestTransaction(t)
						if _, exists := pool.Get(tx.Hash()); !exists {
							// Not an error, transaction might not exist
						}
					case 2:
						tx := createTestTransaction(t)
						pool.Remove(tx.Hash())
					}
				}
			}(i)
		}

		// Wait for completion
		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			require.NoError(t, err)
		}
	})

	t.Run("Recovery After Restart", func(t *testing.T) {
		// Create first pool instance
		pool1, err := NewMMapPool(poolPath)
		require.NoError(t, err)

		// Add test transactions
		txs := make([]*types.Transaction, 0)
		for i := 0; i < 10; i++ {
			tx := createTestTransaction(t)
			txs = append(txs, tx)
			err := pool1.Add(tx)
			require.NoError(t, err)
		}

		// Close first instance
		pool1.Close()

		// Create second pool instance
		pool2, err := NewMMapPool(poolPath)
		require.NoError(t, err)
		defer pool2.Close()

		// Verify transactions persisted
		for _, tx := range txs {
			retrievedTx, exists := pool2.Get(tx.Hash())
			require.True(t, exists)
			require.Equal(t, tx.Hash(), retrievedTx.Hash())
		}
	})
}

func BenchmarkMMapPool(b *testing.B) {
	// Create temporary file for testing
	tmpDir, err := os.MkdirTemp("", "mmap_pool_bench")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	poolPath := filepath.Join(tmpDir, "bench.mmap")
	pool, err := NewMMapPool(poolPath)
	require.NoError(b, err)
	defer pool.Close()

	b.Run("Sequential Add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tx := createTestTransaction(b)
			err := pool.Add(tx)
			require.NoError(b, err)
		}
	})

	b.Run("Sequential Get", func(b *testing.B) {
		// Pre-populate pool
		tx := createTestTransaction(b)
		err := pool.Add(tx)
		require.NoError(b, err)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pool.Get(tx.Hash())
		}
	})

	b.Run("Parallel Add", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				tx := createTestTransaction(b)
				err := pool.Add(tx)
				require.NoError(b, err)
			}
		})
	})

	b.Run("Parallel Get", func(b *testing.B) {
		// Pre-populate pool
		tx := createTestTransaction(b)
		err := pool.Add(tx)
		require.NoError(b, err)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = pool.Get(tx.Hash())
			}
		})
	})

	b.Run("Mixed Operations", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				switch b.N % 3 {
				case 0:
					tx := createTestTransaction(b)
					_ = pool.Add(tx)
				case 1:
					tx := createTestTransaction(b)
					_, _ = pool.Get(tx.Hash())
				case 2:
					tx := createTestTransaction(b)
					_ = pool.Remove(tx.Hash())
				}
			}
		})
	})
}

// Helper function to create test transactions
func createTestTransaction(t testing.TB) *types.Transaction {
	// Generate random values for transaction
	nonce := uint64(time.Now().UnixNano())
	value := big.NewInt(0)
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(1000000000)

	// Generate random addresses
	var toAddr common.Address
	rand.Read(toAddr[:])

	// Create transaction
	tx := types.NewTransaction(nonce, toAddr, value, gasLimit, gasPrice, nil)

	return tx
}
