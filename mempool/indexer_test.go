package mempool

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMempoolIndexer(t *testing.T) {
	logger := zap.NewExample()
	config := &IndexConfig{
		MaxSize:       1000,
		EvictionTime:  time.Hour,
		PruneInterval: time.Minute,
	}

	indexer, err := NewMempoolIndexer(config, logger)
	require.NoError(t, err)
	require.NotNil(t, indexer)

	// Create test transactions with different gas prices
	gasPrice1 := big.NewInt(1e9) // 1 Gwei
	gasPrice2 := big.NewInt(2e9) // 2 Gwei

	tx1 := types.NewTx(&types.DynamicFeeTx{
		ChainID:    big.NewInt(1),
		Nonce:      0,
		GasTipCap:  gasPrice1,
		GasFeeCap:  gasPrice1,
		Gas:        21000,
		To:         &common.Address{1},
		Value:      big.NewInt(1),
		Data:       nil,
	})

	tx2 := types.NewTx(&types.DynamicFeeTx{
		ChainID:    big.NewInt(1),
		Nonce:      1,
		GasTipCap:  gasPrice2,
		GasFeeCap:  gasPrice2,
		Gas:        21000,
		To:         &common.Address{2},
		Value:      big.NewInt(1),
		Data:       nil,
	})

	// Test indexing transactions
	err = indexer.IndexTransaction(tx1)
	require.NoError(t, err)
	err = indexer.IndexTransaction(tx2)
	require.NoError(t, err)

	// Test retrieving transactions
	retrieved := indexer.GetByHash(tx1.Hash().String())
	assert.Equal(t, tx1.Hash(), retrieved.Hash())

	// Test highest priority transaction
	highest := indexer.GetHighestPriority()
	assert.Equal(t, tx2.Hash(), highest.Hash())

	// Test pruning
	time.Sleep(time.Millisecond * 100)
	indexer.pruneOldTransactions()

	// Transactions should still be there as they haven't expired
	retrieved = indexer.GetByHash(tx1.Hash().String())
	assert.NotNil(t, retrieved)
}

func TestMempoolIndexerPruning(t *testing.T) {
	logger := zap.NewExample()
	config := &IndexConfig{
		MaxSize:       1000,
		EvictionTime:  time.Millisecond * 100,
		PruneInterval: time.Millisecond * 50,
	}

	indexer, err := NewMempoolIndexer(config, logger)
	require.NoError(t, err)

	// Create test transaction
	gasPrice := big.NewInt(1e9) // 1 Gwei
	tx1 := types.NewTx(&types.DynamicFeeTx{
		ChainID:    big.NewInt(1),
		Nonce:      0,
		GasTipCap:  gasPrice,
		GasFeeCap:  gasPrice,
		Gas:        21000,
		To:         &common.Address{1},
		Value:      big.NewInt(1),
		Data:       nil,
	})

	// Index transaction
	err = indexer.IndexTransaction(tx1)
	require.NoError(t, err)

	// Wait for eviction time
	time.Sleep(time.Millisecond * 200)

	// Manually trigger pruning
	indexer.pruneOldTransactions()

	// Transaction should be gone
	retrieved := indexer.GetByHash(tx1.Hash().String())
	assert.Nil(t, retrieved)
}

func BenchmarkMempoolIndexer(b *testing.B) {
	logger := zap.NewExample()
	config := &IndexConfig{
		MaxSize:       10000,
		EvictionTime:  time.Hour,
		PruneInterval: time.Minute,
	}

	indexer, err := NewMempoolIndexer(config, logger)
	require.NoError(b, err)

	// Create a test transaction
	gasPrice := big.NewInt(1e9) // 1 Gwei
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:    big.NewInt(1),
		Nonce:      0,
		GasTipCap:  gasPrice,
		GasFeeCap:  gasPrice,
		Gas:        21000,
		To:         &common.Address{1},
		Value:      big.NewInt(1),
		Data:       nil,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := indexer.IndexTransaction(tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
