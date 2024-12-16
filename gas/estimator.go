package gas

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

// Estimator provides gas price estimation and tracking
type Estimator struct {
	client       *ethclient.Client
	logger       *zap.Logger
	baseGasPrice *big.Int
	priorityFee  *big.Int
	mu           sync.RWMutex
	updateTicker *time.Ticker
}

// NewEstimator creates a new gas estimator
func NewEstimator(client *ethclient.Client, logger *zap.Logger) *Estimator {
	e := &Estimator{
		client:       client,
		logger:       logger,
		updateTicker: time.NewTicker(time.Second), // Update every second
	}
	go e.updateLoop()
	return e
}

// updateLoop continuously updates gas prices
func (e *Estimator) updateLoop() {
	for range e.updateTicker.C {
		if err := e.update(); err != nil {
			e.logger.Error("Failed to update gas prices", zap.Error(err))
		}
	}
}

// update fetches latest gas prices
func (e *Estimator) update() error {
	ctx := context.Background()

	// Get base fee
	block, err := e.client.BlockByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}
	baseFee := block.BaseFee()

	// Get priority fee suggestion
	priorityFee, err := e.client.SuggestGasTipCap(ctx)
	if err != nil {
		return fmt.Errorf("failed to get priority fee: %w", err)
	}

	e.mu.Lock()
	e.baseGasPrice = baseFee
	e.priorityFee = priorityFee
	e.mu.Unlock()

	return nil
}

// EstimateGasCost estimates the gas cost for a transaction
func (e *Estimator) EstimateGasCost(ctx context.Context, gasLimit uint64) (*big.Int, error) {
	e.mu.RLock()
	baseFee := new(big.Int).Set(e.baseGasPrice)
	priorityFee := new(big.Int).Set(e.priorityFee)
	e.mu.RUnlock()

	// Calculate total gas price (base fee + priority fee)
	totalGasPrice := new(big.Int).Add(baseFee, priorityFee)

	// Calculate total cost
	gasLimitBig := new(big.Int).SetUint64(gasLimit)
	totalCost := new(big.Int).Mul(totalGasPrice, gasLimitBig)

	return totalCost, nil
}

// EstimateArbitrageGas estimates gas for a typical arbitrage transaction
func (e *Estimator) EstimateArbitrageGas(numHops int) uint64 {
	// Base cost for transaction
	baseCost := uint64(21000)
	
	// Cost per DEX hop (approximate)
	// This includes:
	// - Storage reads (~2000)
	// - Token transfers (~50000)
	// - Swap execution (~100000)
	costPerHop := uint64(152000)

	return baseCost + (costPerHop * uint64(numHops))
}

// Stop stops the gas price updates
func (e *Estimator) Stop() {
	e.updateTicker.Stop()
}
