package uniswap

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// IUniswapV2Pair represents the interface for Uniswap V2 pair contracts
type IUniswapV2Pair interface {
	GetReserves() (reserve0 *big.Int, reserve1 *big.Int, err error)
	Token0() (common.Address, error)
	Token1() (common.Address, error)
	GetAmountOut(amountIn *big.Int, reserveIn *big.Int, reserveOut *big.Int) *big.Int
	GetAmountIn(amountOut *big.Int, reserveIn *big.Int, reserveOut *big.Int) *big.Int
}

// IUniswapV2Router represents the interface for Uniswap V2 router contracts
type IUniswapV2Router interface {
	SwapExactTokensForTokens(amountIn *big.Int, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) error
	GetAmountsOut(amountIn *big.Int, path []common.Address) ([]*big.Int, error)
	GetAmountsIn(amountOut *big.Int, path []common.Address) ([]*big.Int, error)
}
