// Package utils provides utility functions for MEV operations
package utils

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/michaelpento.lv/mevbot/config"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flashbots/go-boost-utils/bls"
	"go.uber.org/zap"
)

// Bundle represents a bundle of transactions
type Bundle struct {
	Txs         []*types.Transaction
	TargetBlock uint64
	Timestamp   time.Time
	Hash        common.Hash
}

// BundleStats contains statistics about a bundle
type BundleStats struct {
	TxCount     int
	TotalGas    uint64
	TotalValue  *big.Int
	TargetBlock uint64
}

// Bundler manages transaction bundles for MEV extraction
type Bundler struct {
	cfg    *config.Config
	client *ethclient.Client
	logger *zap.Logger
	mu     sync.RWMutex
}

// NewBundler creates a new bundler instance
func NewBundler(cfg *config.Config) (*Bundler, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if cfg.Client == nil {
		return nil, fmt.Errorf("eth client is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	return &Bundler{
		cfg:    cfg,
		client: cfg.Client,
		logger: cfg.Logger,
	}, nil
}

// SubmitBundle submits a bundle of transactions to Flashbots
func (b *Bundler) SubmitBundle(ctx context.Context, txs []*types.Transaction) error {
	if len(txs) == 0 {
		return fmt.Errorf("empty transaction bundle")
	}

	// Get current block number
	blockNumber, err := b.client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get block number: %w", err)
	}

	// Create bundle
	bundle, err := b.createBundle(txs, blockNumber+1)
	if err != nil {
		return fmt.Errorf("failed to create bundle: %w", err)
	}

	// Submit to builders
	if err := b.submitToBuilders(ctx, bundle); err != nil {
		return fmt.Errorf("failed to submit to builders: %w", err)
	}

	return nil
}

// createBundle creates a new transaction bundle
func (b *Bundler) createBundle(txs []*types.Transaction, targetBlock uint64) (*Bundle, error) {
	bundle := &Bundle{
		Txs:         txs,
		TargetBlock: targetBlock,
		Timestamp:   time.Now(),
	}

	// Calculate bundle hash
	hash, err := b.calculateBundleHash(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate bundle hash: %w", err)
	}
	bundle.Hash = hash

	return bundle, nil
}

// calculateBundleHash calculates the hash of a bundle
func (b *Bundler) calculateBundleHash(bundle *Bundle) (common.Hash, error) {
	if len(bundle.Txs) == 0 {
		return common.Hash{}, fmt.Errorf("empty bundle")
	}

	// Concatenate transaction hashes
	data := []byte{}
	for _, tx := range bundle.Txs {
		data = append(data, tx.Hash().Bytes()...)
	}

	// Add target block and timestamp
	blockBytes := new(big.Int).SetUint64(bundle.TargetBlock).Bytes()
	data = append(data, blockBytes...)
	data = append(data, new(big.Int).SetInt64(bundle.Timestamp.Unix()).Bytes()...)

	return common.BytesToHash(data), nil
}

// submitToBuilders submits the bundle to multiple block builders
func (b *Bundler) submitToBuilders(ctx context.Context, bundle *Bundle) error {
	// Sign bundle
	signature, err := b.signBundle(bundle)
	if err != nil {
		return fmt.Errorf("failed to sign bundle: %w", err)
	}

	// Submit to each builder
	var wg sync.WaitGroup
	errors := make(chan error, len(b.getBuilderURLs()))

	for _, url := range b.getBuilderURLs() {
		wg.Add(1)
		go func(builderURL string) {
			defer wg.Done()
			if err := b.submitToBuilder(ctx, bundle, signature, builderURL); err != nil {
				errors <- fmt.Errorf("failed to submit to builder %s: %w", builderURL, err)
			}
		}(url)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

// signBundle signs a bundle with the Flashbots private key
func (b *Bundler) signBundle(bundle *Bundle) ([]byte, error) {
	// Parse private key
	privateKey, err := bls.SecretKeyFromBytes(common.FromHex(b.cfg.FlashbotsKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Sign bundle hash
	signature := bls.Sign(privateKey, bundle.Hash.Bytes())
	sigBytes := signature.Bytes()
	return sigBytes[:], nil
}

// submitToBuilder submits a bundle to a specific builder
func (b *Bundler) submitToBuilder(ctx context.Context, bundle *Bundle, signature []byte, builderURL string) error {
	// TODO: Implement builder-specific submission logic
	return nil
}

// getBuilderURLs returns a list of active block builder URLs
func (b *Bundler) getBuilderURLs() []string {
	return []string{b.cfg.FlashbotsRPC}
}

// GetBundleStats returns statistics about a bundle
func (b *Bundler) GetBundleStats(bundle *Bundle) *BundleStats {
	stats := &BundleStats{
		TxCount:     len(bundle.Txs),
		TargetBlock: bundle.TargetBlock,
		TotalValue:  new(big.Int),
		TotalGas:    0,
	}

	for _, tx := range bundle.Txs {
		stats.TotalGas += tx.Gas()
		stats.TotalValue.Add(stats.TotalValue, tx.Value())
	}

	return stats
}
