package utils

import (
	"errors"
	"math/big"

	"github.com/michaelpento.lv/mevbot/config"
)

// ProfitCalculator calculates potential profits for MEV opportunities
type ProfitCalculator struct {
	config *config.Config
}

// NewProfitCalculator creates a new profit calculator
func NewProfitCalculator(cfg *config.Config) *ProfitCalculator {
	return &ProfitCalculator{
		config: cfg,
	}
}

// CalculateOptimalFrontrunAmount calculates the optimal amount for a frontrun transaction
func (p *ProfitCalculator) CalculateOptimalFrontrunAmount(pool *config.PoolConfig, victimAmount *big.Int) (*big.Int, error) {
	if p.config == nil {
		return nil, errors.New("profit calculator not initialized with config")
	}
	if pool == nil || victimAmount == nil {
		return nil, errors.New("invalid parameters")
	}

	// Start with a percentage of victim's amount
	optimalAmount := new(big.Int).Div(
		new(big.Int).Mul(victimAmount, big.NewInt(20)), // 20%
		big.NewInt(100),
	)

	return optimalAmount, nil
}

// CalculateExpectedProfit calculates expected profit for a sandwich attack
func (p *ProfitCalculator) CalculateExpectedProfit(
	pool *config.PoolConfig,
	frontrunAmount *big.Int,
	victimAmount *big.Int,
) (*big.Int, error) {
	if p.config == nil {
		return nil, errors.New("profit calculator not initialized with config")
	}
	if pool == nil || frontrunAmount == nil || victimAmount == nil {
		return nil, errors.New("invalid parameters")
	}

	// Calculate price impact
	priceImpact := p.calculatePriceImpact(pool.Reserve0, pool.Reserve1, frontrunAmount)
	if priceImpact == nil {
		return nil, errors.New("failed to calculate price impact")
	}

	// Calculate expected output amount
	outputAmount := p.calculateOutputAmount(pool.Reserve0, pool.Reserve1, frontrunAmount)
	if outputAmount == nil {
		return nil, errors.New("failed to calculate output amount")
	}

	// Calculate gas cost
	gasCost := p.calculateGasCost(p.config.GasPrice)
	if gasCost == nil {
		return nil, errors.New("failed to calculate gas cost")
	}

	// Calculate profit (output - input - gas)
	profit := new(big.Int).Sub(outputAmount, frontrunAmount)
	profit.Sub(profit, gasCost)

	return profit, nil
}

// calculatePriceImpact calculates the price impact of a trade
func (p *ProfitCalculator) calculatePriceImpact(reserve0, reserve1, amount *big.Int) *big.Int {
	if reserve0 == nil || reserve1 == nil || amount == nil {
		return nil
	}

	impact := new(big.Int).Mul(amount, big.NewInt(10000))
	impact.Div(impact, reserve0)

	return impact
}

// calculateOutputAmount calculates the output amount for a given input
func (p *ProfitCalculator) calculateOutputAmount(reserve0, reserve1, amountIn *big.Int) *big.Int {
	if reserve0 == nil || reserve1 == nil || amountIn == nil {
		return nil
	}

	// Using constant product formula: x * y = k
	numerator := new(big.Int).Mul(amountIn, reserve1)
	denominator := new(big.Int).Add(reserve0, amountIn)
	
	return new(big.Int).Div(numerator, denominator)
}

// calculateGasCost calculates the gas cost for a transaction
func (p *ProfitCalculator) calculateGasCost(gasPrice *big.Int) *big.Int {
	if gasPrice == nil {
		return nil
	}

	// Estimate gas used for sandwich (frontrun + backrun)
	gasUsed := big.NewInt(300000) // Conservative estimate
	return new(big.Int).Mul(gasPrice, gasUsed)
}
