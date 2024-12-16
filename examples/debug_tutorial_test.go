package examples

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateProfit(t *testing.T) {
	tests := []struct {
		name      string
		arb       *SimpleArbitrage
		wantError bool
	}{
		{
			name: "valid calculation",
			arb: &SimpleArbitrage{
				TokenA:   "ETH",
				TokenB:   "USDC",
				AmountIn: big.NewInt(1000000000), // 1 ETH in wei
			},
			wantError: false,
		},
		{
			name: "zero amount",
			arb: &SimpleArbitrage{
				TokenA:   "ETH",
				TokenB:   "USDC",
				AmountIn: big.NewInt(0),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := CalculateProfit(ctx, tt.arb)

			if tt.wantError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, tt.arb.AmountOut)
			assert.NotNil(t, tt.arb.Profit)
			assert.Greater(t, tt.arb.Profit.Int64(), int64(0))
		})
	}
}

func TestExecuteArbitrage(t *testing.T) {
	ctx := context.Background()
	
	// Test case 1: Profitable trade
	profitableArb := &SimpleArbitrage{
		TokenA:   "ETH",
		TokenB:   "USDC",
		AmountIn: big.NewInt(1000000000),
		Profit:   big.NewInt(2000000), // Above minimum threshold
		Path:     []string{"Step 1", "Step 2", "Step 3"},
	}
	
	err := ExecuteArbitrage(ctx, profitableArb)
	assert.NoError(t, err)

	// Test case 2: Unprofitable trade
	unprofitableArb := &SimpleArbitrage{
		TokenA:   "ETH",
		TokenB:   "USDC",
		AmountIn: big.NewInt(1000000000),
		Profit:   big.NewInt(100000), // Below minimum threshold
		Path:     []string{"Step 1", "Step 2", "Step 3"},
	}
	
	err = ExecuteArbitrage(ctx, unprofitableArb)
	assert.Error(t, err)
}
