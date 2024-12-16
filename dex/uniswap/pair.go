package uniswap

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// UniswapV2Pair represents a Uniswap V2 pair contract
type UniswapV2Pair struct {
	contract *bind.BoundContract
	address  common.Address
	client   *ethclient.Client
	pairABI  abi.ABI
}

// Pair contract ABI
const pairABIJson = `[{
	"constant": true,
	"inputs": [],
	"name": "getReserves",
	"outputs": [
		{"name": "reserve0", "type": "uint112"},
		{"name": "reserve1", "type": "uint112"},
		{"name": "blockTimestampLast", "type": "uint32"}
	],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}, {
	"constant": true,
	"inputs": [],
	"name": "token0",
	"outputs": [{"name": "", "type": "address"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}, {
	"constant": true,
	"inputs": [],
	"name": "token1",
	"outputs": [{"name": "", "type": "address"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}]`

// NewUniswapV2Pair creates a new UniswapV2Pair instance
func NewUniswapV2Pair(address common.Address, client *ethclient.Client) (*UniswapV2Pair, error) {
	parsedABI, err := abi.JSON(strings.NewReader(pairABIJson))
	if err != nil {
		return nil, fmt.Errorf("failed to parse pair ABI: %w", err)
	}

	contract := bind.NewBoundContract(address, parsedABI, client, client, client)

	return &UniswapV2Pair{
		contract: contract,
		address:  address,
		client:   client,
		pairABI:  parsedABI,
	}, nil
}

// GetReserves returns the current reserves of the pair
func (p *UniswapV2Pair) GetReserves() (reserve0 *big.Int, reserve1 *big.Int, err error) {
	var out []interface{}
	err = p.contract.Call(&bind.CallOpts{}, &out, "getReserves")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get reserves: %w", err)
	}

	// Parse results
	reserve0, ok := out[0].(*big.Int)
	if !ok {
		return nil, nil, fmt.Errorf("failed to parse reserve0")
	}
	reserve1, ok := out[1].(*big.Int)
	if !ok {
		return nil, nil, fmt.Errorf("failed to parse reserve1")
	}

	return reserve0, reserve1, nil
}

// Token0 returns the address of token0
func (p *UniswapV2Pair) Token0() (common.Address, error) {
	var out []interface{}
	err := p.contract.Call(&bind.CallOpts{}, &out, "token0")
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to get token0: %w", err)
	}

	addr, ok := out[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("failed to parse token0 address")
	}

	return addr, nil
}

// Token1 returns the address of token1
func (p *UniswapV2Pair) Token1() (common.Address, error) {
	var out []interface{}
	err := p.contract.Call(&bind.CallOpts{}, &out, "token1")
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to get token1: %w", err)
	}

	addr, ok := out[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("failed to parse token1 address")
	}

	return addr, nil
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
