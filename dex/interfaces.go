package dex

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Exchange represents a decentralized exchange
type Exchange interface {
	// GetName returns the exchange name
	GetName() string

	// GetPrice returns the price of token1 in terms of token0
	GetPrice(ctx context.Context, token0, token1 common.Address) (*big.Int, error)

	// GetReserves returns the reserves of a token pair
	GetReserves(ctx context.Context, token0, token1 common.Address) (*Reserves, error)

	// EstimateReturn estimates the return amount for a swap
	EstimateReturn(ctx context.Context, amountIn *big.Int, path []common.Address) (*big.Int, error)

	// GetAmountIn calculates required input amount for desired output
	GetAmountIn(ctx context.Context, amountOut *big.Int, path []common.Address) (*big.Int, error)
}

// RouterProvider defines an interface for exchanges that provide router contracts
type RouterProvider interface {
	GetRouterAddress() common.Address
}

// Reserves represents token pair reserves
type Reserves struct {
	Reserve0    *big.Int
	Reserve1    *big.Int
	BlockNumber uint32
}
