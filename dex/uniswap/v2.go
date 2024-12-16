package uniswap

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/michaelpento.lv/mevbot/dex"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Contract addresses
var (
	MainnetRouter  = common.HexToAddress("0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D")
	MainnetFactory = common.HexToAddress("0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f")
	WETHAddress    = common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
)

// UniswapV2 implements the Exchange interface for Uniswap V2
type UniswapV2 struct {
	client     *ethclient.Client
	factory    common.Address
	router     common.Address
	initCode   []byte
	pairs      map[common.Address]*UniswapV2Pair
	pairABI    abi.ABI
}

// NewUniswapV2 creates a new Uniswap V2 exchange
func NewUniswapV2(client *ethclient.Client) (*UniswapV2, error) {
	parsedABI, err := abi.JSON(strings.NewReader(pairABIJson))
	if err != nil {
		return nil, fmt.Errorf("failed to parse pair ABI: %w", err)
	}

	return &UniswapV2{
		client:     client,
		factory:    MainnetFactory,
		router:     MainnetRouter,
		initCode:   common.FromHex("0x96e8ac4277198ff8b6f785478aa9a39f403cb768dd02cbee326c3e7da348845f"),
		pairs:      make(map[common.Address]*UniswapV2Pair),
		pairABI:    parsedABI,
	}, nil
}

// GetName returns the exchange name
func (u *UniswapV2) GetName() string {
	return "UniswapV2"
}

// GetRouterAddress returns the router contract address
func (u *UniswapV2) GetRouterAddress() common.Address {
	return u.router
}

// GetPrice returns the price of token1 in terms of token0
func (u *UniswapV2) GetPrice(ctx context.Context, token0, token1 common.Address) (*big.Int, error) {
	reserves, err := u.GetReserves(ctx, token0, token1)
	if err != nil {
		return nil, fmt.Errorf("failed to get reserves: %w", err)
	}

	if reserves.Reserve0.Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("insufficient liquidity")
	}

	// Calculate price (reserve1/reserve0)
	price := new(big.Int).Div(
		new(big.Int).Mul(reserves.Reserve1, big.NewInt(1e18)),
		reserves.Reserve0,
	)

	return price, nil
}

// GetReserves returns the reserves of a token pair
func (u *UniswapV2) GetReserves(ctx context.Context, token0, token1 common.Address) (*dex.Reserves, error) {
	pair, err := u.getPair(ctx, token0, token1)
	if err != nil {
		return nil, err
	}

	reserve0, reserve1, err := pair.GetReserves()
	if err != nil {
		return nil, fmt.Errorf("failed to get reserves: %w", err)
	}

	return &dex.Reserves{
		Reserve0:    reserve0,
		Reserve1:    reserve1,
		BlockNumber: 0, // We don't need block timestamp for our purposes
	}, nil
}

// EstimateReturn estimates the return amount for a swap
func (u *UniswapV2) EstimateReturn(ctx context.Context, amountIn *big.Int, path []common.Address) (*big.Int, error) {
	if len(path) < 2 {
		return nil, fmt.Errorf("invalid path length")
	}

	amounts := make([]*big.Int, len(path))
	amounts[0] = amountIn

	// For each pair in path, calculate output amount
	for i := 0; i < len(path)-1; i++ {
		reserves, err := u.GetReserves(ctx, path[i], path[i+1])
		if err != nil {
			return nil, err
		}

		amounts[i+1] = u.getAmountOut(amounts[i], reserves.Reserve0, reserves.Reserve1)
	}

	return amounts[len(amounts)-1], nil
}

// GetAmountIn calculates required input amount for desired output
func (u *UniswapV2) GetAmountIn(ctx context.Context, amountOut *big.Int, path []common.Address) (*big.Int, error) {
	if len(path) < 2 {
		return nil, fmt.Errorf("invalid path length")
	}

	amounts := make([]*big.Int, len(path))
	amounts[len(amounts)-1] = amountOut

	// For each pair in path, calculate input amount (in reverse)
	for i := len(path) - 1; i > 0; i-- {
		reserves, err := u.GetReserves(ctx, path[i-1], path[i])
		if err != nil {
			return nil, err
		}

		amounts[i-1] = u.getAmountIn(amounts[i], reserves.Reserve0, reserves.Reserve1)
	}

	return amounts[0], nil
}

// getPair returns the pair contract for two tokens
func (u *UniswapV2) getPair(ctx context.Context, token0, token1 common.Address) (*UniswapV2Pair, error) {
	pairAddr := u.pairFor(token0, token1)
	if pair, ok := u.pairs[pairAddr]; ok {
		return pair, nil
	}

	pair, err := NewUniswapV2Pair(pairAddr, u.client)
	if err != nil {
		return nil, fmt.Errorf("failed to create pair contract: %w", err)
	}

	u.pairs[pairAddr] = pair
	return pair, nil
}

// pairFor calculates the pair address for two tokens
func (u *UniswapV2) pairFor(token0, token1 common.Address) common.Address {
	if token0.Hex() > token1.Hex() {
		token0, token1 = token1, token0
	}

	salt := crypto.Keccak256(token0.Bytes(), token1.Bytes())
	return common.BytesToAddress(crypto.Keccak256([]byte{
		0xff,
	}, u.factory.Bytes(), salt, u.initCode))
}

// getAmountOut calculates output amount for an input amount
func (u *UniswapV2) getAmountOut(amountIn, reserveIn, reserveOut *big.Int) *big.Int {
	amountInWithFee := new(big.Int).Mul(amountIn, big.NewInt(997))
	numerator := new(big.Int).Mul(amountInWithFee, reserveOut)
	denominator := new(big.Int).Add(
		new(big.Int).Mul(reserveIn, big.NewInt(1000)),
		amountInWithFee,
	)
	return new(big.Int).Div(numerator, denominator)
}

// getAmountIn calculates input amount for a desired output amount
func (u *UniswapV2) getAmountIn(amountOut, reserveIn, reserveOut *big.Int) *big.Int {
	numerator := new(big.Int).Mul(
		new(big.Int).Mul(reserveIn, amountOut),
		big.NewInt(1000),
	)
	denominator := new(big.Int).Mul(
		new(big.Int).Sub(reserveOut, amountOut),
		big.NewInt(997),
	)
	return new(big.Int).Add(
		new(big.Int).Div(numerator, denominator),
		big.NewInt(1),
	)
}
