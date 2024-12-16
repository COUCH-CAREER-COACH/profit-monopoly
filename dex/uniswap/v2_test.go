package uniswap

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestGetAmountOut(t *testing.T) {
	// Test data
	amountIn := big.NewInt(1000000000000000000) // 1 ETH
	reserveIn := big.NewInt(10000000000000000000) // 10 ETH
	reserveOut := big.NewInt(5000000000) // 5000 USDC (6 decimals)
	
	provider := NewV2Provider(common.Address{})
	amountOut := provider.GetAmountOut(amountIn, reserveIn, reserveOut)
	
	// Basic assertions
	assert.NotNil(t, amountOut)
	assert.True(t, amountOut.Sign() > 0)
}

func TestGetOptimalAmount(t *testing.T) {
	provider := NewV2Provider(common.Address{})
	
	// Test data
	reserveIn := big.NewInt(10000000000000000000) // 10 ETH
	reserveOut := big.NewInt(5000000000) // 5000 USDC
	
	amount := provider.GetOptimalAmount(reserveIn, reserveOut)
	
	// Basic assertions
	assert.NotNil(t, amount)
	assert.True(t, amount.Cmp(reserveIn) < 0) // Optimal amount should be less than reserve
}
