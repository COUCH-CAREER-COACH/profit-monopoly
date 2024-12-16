package arbitrage

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestDetectArbitrage(t *testing.T) {
	detector := NewDetector()

	// Mock exchange rates
	// Exchange 1: 1 ETH = 2000 USDC
	// Exchange 2: 1 ETH = 2010 USDC
	exchange1Rates := map[common.Address]map[common.Address]*big.Int{
		common.HexToAddress("0xETH"): {
			common.HexToAddress("0xUSDC"): big.NewInt(2000000000), // 2000 USDC
		},
	}

	exchange2Rates := map[common.Address]map[common.Address]*big.Int{
		common.HexToAddress("0xETH"): {
			common.HexToAddress("0xUSDC"): big.NewInt(2010000000), // 2010 USDC
		},
	}

	// Detect arbitrage opportunities
	opportunities := detector.DetectOpportunities(exchange1Rates, exchange2Rates)

	// Basic assertions
	assert.NotNil(t, opportunities)
	if len(opportunities) > 0 {
		// Verify the first opportunity
		opp := opportunities[0]
		assert.True(t, opp.ExpectedProfit.Sign() > 0)
		assert.NotNil(t, opp.Route)
	}
}
