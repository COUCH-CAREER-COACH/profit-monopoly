package uniswap

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// UniswapV2Pair represents a Uniswap V2 pair contract
type UniswapV2Pair struct {
	contract *bind.BoundContract
	address  common.Address
	client   *ethclient.Client
}

// NewUniswapV2Pair creates a new UniswapV2Pair instance
func NewUniswapV2Pair(address common.Address, client *ethclient.Client) (*UniswapV2Pair, error) {
	contract, err := bind.NewBoundContract(address, PairABI, client, client, client)
	if err != nil {
		return nil, err
	}

	return &UniswapV2Pair{
		contract: contract,
		address:  address,
		client:   client,
	}, nil
}

// GetReserves returns the current reserves of the pair
func (p *UniswapV2Pair) GetReserves() (reserve0 *big.Int, reserve1 *big.Int, err error) {
	var result struct {
		Reserve0           *big.Int
		Reserve1           *big.Int
		BlockTimestampLast uint32
	}

	err = p.contract.Call(nil, &result, "getReserves")
	if err != nil {
		return nil, nil, err
	}

	return result.Reserve0, result.Reserve1, nil
}

// Token0 returns the address of token0
func (p *UniswapV2Pair) Token0() (common.Address, error) {
	var result common.Address
	err := p.contract.Call(nil, &result, "token0")
	return result, err
}

// Token1 returns the address of token1
func (p *UniswapV2Pair) Token1() (common.Address, error) {
	var result common.Address
	err := p.contract.Call(nil, &result, "token1")
	return result, err
}

// GetAmountOut calculates the output amount for a given input amount
func (p *UniswapV2Pair) GetAmountOut(amountIn *big.Int, reserveIn *big.Int, reserveOut *big.Int) *big.Int {
	if amountIn.Sign() <= 0 || reserveIn.Sign() <= 0 || reserveOut.Sign() <= 0 {
		return big.NewInt(0)
	}

	amountInWithFee := new(big.Int).Mul(amountIn, big.NewInt(997))
	numerator := new(big.Int).Mul(amountInWithFee, reserveOut)
	denominator := new(big.Int).Add(new(big.Int).Mul(reserveIn, big.NewInt(1000)), amountInWithFee)
	
	return new(big.Int).Div(numerator, denominator)
}

// GetAmountIn calculates the input amount for a given output amount
func (p *UniswapV2Pair) GetAmountIn(amountOut *big.Int, reserveIn *big.Int, reserveOut *big.Int) *big.Int {
	if amountOut.Sign() <= 0 || reserveIn.Sign() <= 0 || reserveOut.Sign() <= 0 {
		return big.NewInt(0)
	}

	numerator := new(big.Int).Mul(new(big.Int).Mul(reserveIn, amountOut), big.NewInt(1000))
	denominator := new(big.Int).Mul(new(big.Int).Sub(reserveOut, amountOut), big.NewInt(997))
	
	amountIn := new(big.Int).Div(numerator, denominator)
	return new(big.Int).Add(amountIn, big.NewInt(1))
}
