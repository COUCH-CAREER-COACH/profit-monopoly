package network

import (
	"container/heap"
	"context"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	NumShards       = 256
	MaxTxsPerShard  = 10000
	CleanupInterval = 5 * time.Minute
	MaxTxAge        = 30 * time.Minute
)

// TxEntry represents a transaction in the index
type TxEntry struct {
	Tx        *types.Transaction
	Timestamp time.Time
	GasPrice  *big.Int
	Nonce     uint64
}

// TxShard represents a shard of the transaction index
type TxShard struct {
	sync.RWMutex
	txs map[common.Hash]*TxEntry
}

// MempoolIndex implements a high-performance transaction index
type MempoolIndex struct {
	shards [NumShards]*TxShard
	logger *zap.Logger
	stats  struct {
		insertions atomic.Uint64
		evictions  atomic.Uint64
		lookups    atomic.Uint64
		collisions atomic.Uint64
	}
}

// NewMempoolIndex creates a new mempool index
func NewMempoolIndex(logger *zap.Logger) *MempoolIndex {
	idx := &MempoolIndex{
		logger: logger,
	}

	// Initialize shards
	for i := 0; i < NumShards; i++ {
		idx.shards[i] = &TxShard{
			txs: make(map[common.Hash]*TxEntry, MaxTxsPerShard),
		}
	}

	// Start cleanup routine
	go idx.cleanup()

	return idx
}

// getShard returns the appropriate shard for a transaction hash
func (m *MempoolIndex) getShard(hash common.Hash) *TxShard {
	// Use xxhash for fast hashing
	h := xxhash.Sum64(hash[:])
	return m.shards[h%NumShards]
}

// getEffectiveGasPrice returns the effective gas price for a transaction
func getEffectiveGasPrice(tx *types.Transaction) *big.Int {
	if tx.Type() == types.DynamicFeeTxType {
		// For EIP-1559 transactions, use the effective gas price
		baseFee := big.NewInt(0) // TODO: Get this from the latest block
		tip := tx.GasTipCap()
		if tip.Cmp(tx.GasFeeCap()) > 0 {
			tip = tx.GasFeeCap()
		}
		return new(big.Int).Add(baseFee, tip)
	}
	return tx.GasPrice()
}

// Insert adds a transaction to the index
func (m *MempoolIndex) Insert(tx *types.Transaction) bool {
	hash := tx.Hash()
	shard := m.getShard(hash)

	shard.Lock()
	defer shard.Unlock()

	// Check if tx already exists
	if _, exists := shard.txs[hash]; exists {
		m.stats.collisions.Add(1)
		return false
	}

	// Check shard capacity
	if len(shard.txs) >= MaxTxsPerShard {
		m.evictOldest(shard)
	}

	// Insert new transaction
	shard.txs[hash] = &TxEntry{
		Tx:        tx,
		Timestamp: time.Now(),
		GasPrice:  getEffectiveGasPrice(tx),
		Nonce:     tx.Nonce(),
	}

	m.stats.insertions.Add(1)
	return true
}

// Get retrieves a transaction from the index
func (m *MempoolIndex) Get(hash common.Hash) *types.Transaction {
	shard := m.getShard(hash)

	shard.RLock()
	defer shard.RUnlock()

	if entry, exists := shard.txs[hash]; exists {
		m.stats.lookups.Add(1)
		return entry.Tx
	}
	return nil
}

// Remove removes a transaction from the index
func (m *MempoolIndex) Remove(hash common.Hash) bool {
	shard := m.getShard(hash)

	shard.Lock()
	defer shard.Unlock()

	if _, exists := shard.txs[hash]; exists {
		delete(shard.txs, hash)
		m.stats.evictions.Add(1)
		return true
	}
	return false
}

// evictOldest removes the oldest transaction from a shard
func (m *MempoolIndex) evictOldest(shard *TxShard) {
	var oldestHash common.Hash
	var oldestTime time.Time

	// Find oldest transaction
	for hash, entry := range shard.txs {
		if oldestTime.IsZero() || entry.Timestamp.Before(oldestTime) {
			oldestTime = entry.Timestamp
			oldestHash = hash
		}
	}

	if !oldestTime.IsZero() {
		delete(shard.txs, oldestHash)
		m.stats.evictions.Add(1)
	}
}

// cleanup periodically removes old transactions
func (m *MempoolIndex) cleanup() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		for _, shard := range m.shards {
			shard.Lock()

			now := time.Now()
			for hash, entry := range shard.txs {
				if now.Sub(entry.Timestamp) > MaxTxAge {
					delete(shard.txs, hash)
					m.stats.evictions.Add(1)
				}
			}

			shard.Unlock()
		}
	}
}

// GetStats returns the current index statistics
func (m *MempoolIndex) GetStats() map[string]uint64 {
	return map[string]uint64{
		"insertions": m.stats.insertions.Load(),
		"evictions":  m.stats.evictions.Load(),
		"lookups":    m.stats.lookups.Load(),
		"collisions": m.stats.collisions.Load(),
	}
}

// TxPriority represents transaction priority in the mempool
type TxPriority struct {
	Transaction *types.Transaction
	GasPrice    *big.Int
	Nonce       uint64
	TimeAdded   time.Time
}

// TxPriorityQueue implements a priority queue for transactions
type TxPriorityQueue []*TxPriority

func NewTxPriorityQueue() *TxPriorityQueue {
	pq := make(TxPriorityQueue, 0)
	heap.Init(&pq)
	return &pq
}

func (pq TxPriorityQueue) Len() int { return len(pq) }

func (pq TxPriorityQueue) Less(i, j int) bool {
	// Higher gas price has higher priority
	return pq[i].GasPrice.Cmp(pq[j].GasPrice) > 0
}

func (pq TxPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *TxPriorityQueue) Push(x interface{}) {
	tx := x.(*types.Transaction)
	item := &TxPriority{
		Transaction: tx,
		GasPrice:    getEffectiveGasPrice(tx),
		Nonce:       tx.Nonce(),
		TimeAdded:   time.Now(),
	}
	*pq = append(*pq, item)
}

func (pq *TxPriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	if n == 0 {
		return nil
	}
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*pq = old[0 : n-1]
	return item.Transaction
}

// MempoolIndexer provides optimized transaction indexing
type MempoolIndexer struct {
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *zap.Logger
	mu      sync.RWMutex
	metrics struct {
		txCount     prometheus.Counter
		queueDepth  prometheus.Gauge
		avgGasPrice prometheus.Gauge
		indexTime   prometheus.Histogram
	}
	txQueue    *TxPriorityQueue
	txByHash   map[string]*types.Transaction
	txByNonce  map[string]map[uint64]*types.Transaction
	maxTxs     uint64
	updateChan chan *types.Transaction
}

// NewMempoolIndexer creates a new mempool indexer
func NewMempoolIndexer(ctx context.Context, logger *zap.Logger, maxTxs uint64) *MempoolIndexer {
	ctx, cancel := context.WithCancel(ctx)
	m := &MempoolIndexer{
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		txQueue:    NewTxPriorityQueue(),
		txByHash:   make(map[string]*types.Transaction),
		txByNonce:  make(map[string]map[uint64]*types.Transaction),
		maxTxs:     maxTxs,
		updateChan: make(chan *types.Transaction, 1000),
	}

	// Initialize metrics
	m.metrics.txCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mempool_tx_count",
		Help: "Number of transactions in mempool",
	})
	m.metrics.queueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mempool_queue_depth",
		Help: "Current depth of transaction queue",
	})
	m.metrics.avgGasPrice = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mempool_avg_gas_price",
		Help: "Average gas price of transactions in mempool",
	})
	m.metrics.indexTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "mempool_index_time",
		Help:    "Time taken to index transactions",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10),
	})

	go m.run()
	return m
}

// AddTransaction adds a transaction to the index
func (m *MempoolIndexer) AddTransaction(tx *types.Transaction) {
	select {
	case m.updateChan <- tx:
	default:
		m.logger.Warn("Transaction update channel full")
	}
}

func (m *MempoolIndexer) run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case tx := <-m.updateChan:
			m.index(tx)
		case <-ticker.C:
			m.updateAverageGasPrice()
			m.prune()
		}
	}
}

func (m *MempoolIndexer) index(tx *types.Transaction) {
	start := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	hash := tx.Hash().Hex()

	// Skip if already indexed
	if _, exists := m.txByHash[hash]; exists {
		return
	}

	// Add to queue
	heap.Push(m.txQueue, tx)
	m.txByHash[hash] = tx

	// Get sender
	signer := types.LatestSignerForChainID(big.NewInt(1)) // TODO: Get chainID from config
	sender, err := types.Sender(signer, tx)
	if err != nil {
		m.logger.Error("Failed to get transaction sender", zap.Error(err))
		return
	}

	// Index by nonce
	from := sender.Hex()
	if m.txByNonce[from] == nil {
		m.txByNonce[from] = make(map[uint64]*types.Transaction)
	}
	m.txByNonce[from][tx.Nonce()] = tx

	// Update metrics
	m.metrics.txCount.Inc()
	m.metrics.queueDepth.Set(float64(m.txQueue.Len()))
	m.metrics.indexTime.Observe(time.Since(start).Seconds())
}

func (m *MempoolIndexer) updateAverageGasPrice() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.txQueue.Len() == 0 {
		m.metrics.avgGasPrice.Set(0)
		return
	}

	var total big.Int
	for _, tx := range m.txByHash {
		total.Add(&total, getEffectiveGasPrice(tx))
	}
	avg := new(big.Int).Div(&total, big.NewInt(int64(len(m.txByHash))))
	m.metrics.avgGasPrice.Set(float64(avg.Uint64()))
}

func (m *MempoolIndexer) prune() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for len(m.txByHash) > int(m.maxTxs) {
		// Remove oldest transaction
		tx := heap.Pop(m.txQueue).(*types.Transaction)
		if tx == nil {
			return
		}

		hash := tx.Hash().Hex()
		delete(m.txByHash, hash)

		// Remove from nonce index
		signer := types.LatestSignerForChainID(big.NewInt(1))
		if sender, err := types.Sender(signer, tx); err == nil {
			from := sender.Hex()
			delete(m.txByNonce[from], tx.Nonce())
			if len(m.txByNonce[from]) == 0 {
				delete(m.txByNonce, from)
			}
		}

		m.metrics.txCount.Add(-1)
		m.metrics.queueDepth.Set(float64(m.txQueue.Len()))
	}
}

// GetHighestGasPriceTx returns the transaction with the highest gas price
func (m *MempoolIndexer) GetHighestGasPriceTx() *types.Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.txQueue.Len() == 0 {
		return nil
	}

	// Peek at the highest priority transaction
	highest := (*m.txQueue)[0]
	return highest.Transaction
}

// GetTxByHash returns a transaction by its hash
func (m *MempoolIndexer) GetTxByHash(hash string) *types.Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.txByHash[hash]
}

// GetTxsByNonce returns all transactions for an address with a specific nonce
func (m *MempoolIndexer) GetTxsByNonce(address string, nonce uint64) []*types.Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if nonceMap, exists := m.txByNonce[address]; exists {
		if tx, exists := nonceMap[nonce]; exists {
			return []*types.Transaction{tx}
		}
	}
	return nil
}

// Cleanup performs cleanup operations
func (m *MempoolIndexer) Cleanup() {
	m.cancel()
}
