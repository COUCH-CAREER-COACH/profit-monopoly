package mempool

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	lru "github.com/hashicorp/golang-lru"
	"go.uber.org/zap"
)

type IndexConfig struct {
	MaxSize       int
	EvictionTime  time.Duration
	PruneInterval time.Duration
}

type MempoolIndexer struct {
	config  *IndexConfig
	logger  *zap.Logger
	cache   *lru.Cache
	txIndex map[string]*types.Transaction
	mu      sync.RWMutex
}

func NewMempoolIndexer(config *IndexConfig, logger *zap.Logger) (*MempoolIndexer, error) {
	cache, err := lru.New(config.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	return &MempoolIndexer{
		config:  config,
		logger:  logger,
		cache:   cache,
		txIndex: make(map[string]*types.Transaction),
	}, nil
}

func (i *MempoolIndexer) IndexTransaction(tx *types.Transaction) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	hash := tx.Hash().String()
	i.txIndex[hash] = tx
	i.cache.Add(hash, tx)

	return nil
}

func (i *MempoolIndexer) GetByHash(hash string) *types.Transaction {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if tx, exists := i.txIndex[hash]; exists {
		return tx
	}
	return nil
}

func (i *MempoolIndexer) GetHighestPriority() *types.Transaction {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var highestPriorityTx *types.Transaction
	var highestPriority *big.Int

	for _, tx := range i.txIndex {
		if highestPriorityTx == nil || tx.GasFeeCap().Cmp(highestPriority) > 0 {
			highestPriorityTx = tx
			highestPriority = tx.GasFeeCap()
		}
	}

	return highestPriorityTx
}

func (i *MempoolIndexer) pruneOldTransactions() {
	i.mu.Lock()
	defer i.mu.Unlock()

	now := time.Now()
	for _, key := range i.cache.Keys() {
		if value, ok := i.cache.Get(key); ok {
			if ts, ok := value.(time.Time); ok {
				if now.Sub(ts) > i.config.EvictionTime {
					i.cache.Remove(key)
					delete(i.txIndex, key.(string))
				}
			}
		}
	}
}

func (i *MempoolIndexer) StartPruning(ctx context.Context) {
	ticker := time.NewTicker(i.config.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			i.pruneOldTransactions()
		}
	}
}
