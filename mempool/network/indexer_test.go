package network

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func createTestTransaction(nonce uint64, gasPrice *big.Int) *types.Transaction {
	privateKey, _ := crypto.GenerateKey()
	signer := types.NewLondonSigner(big.NewInt(1)) // mainnet
	to := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     nonce,
		GasTipCap: gasPrice,
		GasFeeCap: new(big.Int).Mul(gasPrice, big.NewInt(2)),
		Gas:       21000,
		To:        &to,
		Value:     big.NewInt(0),
		Data:      nil,
	})
	signedTx, _ := types.SignTx(tx, signer, privateKey)
	return signedTx
}

func TestMempoolIndexer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()
	indexer := NewMempoolIndexer(ctx, logger, 1000)
	require.NotNil(t, indexer)

	// Test transaction addition and retrieval
	tx1 := createTestTransaction(0, big.NewInt(20000000000))
	tx2 := createTestTransaction(1, big.NewInt(25000000000))
	tx3 := createTestTransaction(2, big.NewInt(15000000000))

	indexer.AddTransaction(tx1)
	indexer.AddTransaction(tx2)
	indexer.AddTransaction(tx3)

	// Give some time for async processing
	time.Sleep(100 * time.Millisecond)

	// Test GetHighestGasPriceTx
	highestTx := indexer.GetHighestGasPriceTx()
	require.NotNil(t, highestTx)
	assert.Equal(t, tx2.Hash(), highestTx.Hash())

	// Test GetTxByHash
	retrievedTx := indexer.GetTxByHash(tx1.Hash().Hex())
	require.NotNil(t, retrievedTx)
	assert.Equal(t, tx1.Hash(), retrievedTx.Hash())

	// Test GetTxsByNonce
	msg, err := tx1.AsMessage(types.NewLondonSigner(big.NewInt(1)), nil)
	require.NoError(t, err)
	txs := indexer.GetTxsByNonce(msg.From().Hex(), 0)
	require.Len(t, txs, 1)
	assert.Equal(t, tx1.Hash(), txs[0].Hash())

	// Test pruning
	for i := uint64(0); i < 1100; i++ {
		tx := createTestTransaction(i, big.NewInt(int64(15000000000+i)))
		indexer.AddTransaction(tx)
	}

	// Give time for pruning
	time.Sleep(100 * time.Millisecond)

	// Verify queue size is maintained
	assert.LessOrEqual(t, indexer.txQueue.Len(), 1000)
}

func BenchmarkMempoolIndexer(b *testing.B) {
	logger := zaptest.NewLogger(b)
	ctx := context.Background()
	indexer := NewMempoolIndexer(ctx, logger, 10000)

	// Create a set of test transactions
	txs := make([]*types.Transaction, 1000)
	for i := range txs {
		txs[i] = createTestTransaction(uint64(i), big.NewInt(int64(20000000000+i)))
	}

	b.ResetTimer()

	// Benchmark transaction addition
	b.Run("AddTransaction", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tx := txs[i%len(txs)]
			indexer.AddTransaction(tx)
		}
	})

	// Benchmark highest gas price retrieval
	b.Run("GetHighestGasPriceTx", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			indexer.GetHighestGasPriceTx()
		}
	})

	// Benchmark transaction lookup by hash
	b.Run("GetTxByHash", func(b *testing.B) {
		tx := txs[0]
		hash := tx.Hash().Hex()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			indexer.GetTxByHash(hash)
		}
	})
}

func TestPriorityQueue(t *testing.T) {
	pq := NewTxPriorityQueue()

	// Add transactions
	tx1 := createTestTransaction(0, big.NewInt(20000000000))
	tx2 := createTestTransaction(1, big.NewInt(25000000000))
	tx3 := createTestTransaction(2, big.NewInt(15000000000))

	pq.Push(tx1)
	pq.Push(tx2)
	pq.Push(tx3)

	// Test pop order (should be highest gas price first)
	highest := pq.Pop()
	assert.Equal(t, tx2.Hash(), highest.Hash())

	second := pq.Pop()
	assert.Equal(t, tx1.Hash(), second.Hash())

	third := pq.Pop()
	assert.Equal(t, tx3.Hash(), third.Hash())

	// Test empty queue
	assert.Nil(t, pq.Pop())
}
