package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Route represents a trading route for arbitrage
type Route struct {
	TokenIn     common.Address
	TokenOut    common.Address
	AmountIn    *big.Int
	AmountOut   *big.Int
	Path        []common.Address
	Exchanges   []common.Address
}

// ArbitrageOpportunity represents a detected arbitrage opportunity
type ArbitrageOpportunity struct {
	Route           *Route
	ExpectedProfit  *big.Int
	RequiredCapital *big.Int
	GasEstimate     uint64
}
