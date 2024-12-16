package sushiswap

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// SushiswapV2 implements the Exchange interface for Sushiswap
type SushiswapV2 struct {
	client     *ethclient.Client
	factory    common.Address
	initCode   []byte
	routerAddr common.Address
	pairABI    abi.ABI
}

// Factory addresses
var (
	MainnetFactory = common.HexToAddress("0xC0AEe478e3658e2610c5F7A4A2E1777cE9e4f2Ac")
	MainnetRouter = common.HexToAddress("0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F")
)

// NewSushiswapV2 creates a new Sushiswap V2 exchange
func NewSushiswapV2(client *ethclient.Client) (*SushiswapV2, error) {
	parsedABI, err := abi.JSON(strings.NewReader(pairABIJson))
	if err != nil {
		return nil, fmt.Errorf("failed to parse pair ABI: %w", err)
	}

	return &SushiswapV2{
		client:     client,
		factory:    MainnetFactory,
		routerAddr: MainnetRouter,
		initCode:   common.FromHex("0xe18a34eb0e04b04f7a0ac29a6e80748dca96319b42c54d679cb821dca90c6303"),
		pairABI:    parsedABI,
	}, nil
}

// GetName returns the exchange name
func (s *SushiswapV2) GetName() string {
	return "SushiswapV2"
}

// EstimateReturn estimates the return amount for a given input
func (s *SushiswapV2) EstimateReturn(ctx context.Context, amountIn *big.Int, path []common.Address) (*big.Int, error) {
	if len(path) < 2 {
		return nil, fmt.Errorf("path must contain at least 2 tokens")
	}

	// Get pair for first two tokens
	pair, err := s.getPair(path[0], path[1])
	if err != nil {
		return nil, fmt.Errorf("failed to get pair: %w", err)
	}

	// Get reserves
	reserve0, reserve1, err := s.getReserves(pair)
	if err != nil {
		return nil, fmt.Errorf("failed to get reserves: %w", err)
	}

	// Calculate amount out
	amountOut := s.getAmountOut(amountIn, reserve0, reserve1)

	// If there are more hops, continue calculating
	for i := 1; i < len(path)-1; i++ {
		pair, err = s.getPair(path[i], path[i+1])
		if err != nil {
			return nil, fmt.Errorf("failed to get pair at hop %d: %w", i, err)
		}

		reserve0, reserve1, err = s.getReserves(pair)
		if err != nil {
			return nil, fmt.Errorf("failed to get reserves at hop %d: %w", i, err)
		}

		amountOut = s.getAmountOut(amountOut, reserve0, reserve1)
	}

	return amountOut, nil
}

// getPair gets the pair address for two tokens
func (s *SushiswapV2) getPair(tokenA, tokenB common.Address) (common.Address, error) {
	// Sort tokens
	token0, token1 := tokenA, tokenB
	if tokenA.Hex() > tokenB.Hex() {
		token0, token1 = tokenB, tokenA
	}

	// Create pair address using CREATE2
	salt := crypto.Keccak256([]byte{
		token0.Bytes()[0], token0.Bytes()[1], token0.Bytes()[2], token0.Bytes()[3],
		token1.Bytes()[0], token1.Bytes()[1], token1.Bytes()[2], token1.Bytes()[3],
	})

	address := common.BytesToAddress(crypto.Keccak256([]byte{
		0xff,
		s.factory.Bytes()[0], s.factory.Bytes()[1], s.factory.Bytes()[2],
		salt[0], salt[1], salt[2],
		s.initCode[0], s.initCode[1], s.initCode[2],
	}))

	return address, nil
}

// getReserves gets the reserves for a pair
func (s *SushiswapV2) getReserves(pair common.Address) (*big.Int, *big.Int, error) {
	contract := bind.NewBoundContract(pair, s.pairABI, s.client, s.client, s.client)

	// Call getReserves
	var out []interface{}
	err := contract.Call(&bind.CallOpts{}, &out, "getReserves")
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

// getAmountOut calculates the output amount for a given input
func (s *SushiswapV2) getAmountOut(amountIn, reserveIn, reserveOut *big.Int) *big.Int {
	if amountIn.Sign() <= 0 || reserveIn.Sign() <= 0 || reserveOut.Sign() <= 0 {
		return big.NewInt(0)
	}

	amountInWithFee := new(big.Int).Mul(amountIn, big.NewInt(997))
	numerator := new(big.Int).Mul(amountInWithFee, reserveOut)
	denominator := new(big.Int).Mul(reserveIn, big.NewInt(1000))
	denominator.Add(denominator, amountInWithFee)

	return new(big.Int).Div(numerator, denominator)
}

// GetAmountIn calculates required input amount for desired output
func (s *SushiswapV2) GetAmountIn(ctx context.Context, amountOut *big.Int, path []common.Address) (*big.Int, error) {
	if len(path) < 2 {
		return nil, fmt.Errorf("path must contain at least 2 tokens")
	}

	amountIn := amountOut
	for i := len(path) - 1; i > 0; i-- {
		tokenIn := path[i-1]
		tokenOut := path[i]

		pair, err := s.getPair(tokenIn, tokenOut)
		if err != nil {
			return nil, fmt.Errorf("failed to get pair address: %w", err)
		}

		reserveIn, reserveOut, err := s.getReserves(pair)
		if err != nil {
			return nil, fmt.Errorf("failed to get reserves: %w", err)
		}

		// Calculate required input amount
		if amountOut.Sign() <= 0 || reserveIn.Sign() <= 0 || reserveOut.Sign() <= 0 {
			return nil, fmt.Errorf("invalid amounts or reserves")
		}

		numerator := new(big.Int).Mul(reserveIn, amountOut)
		numerator.Mul(numerator, big.NewInt(1000))
		
		denominator := new(big.Int).Sub(reserveOut, amountOut)
		denominator.Mul(denominator, big.NewInt(997))
		
		if denominator.Sign() <= 0 {
			return nil, fmt.Errorf("insufficient liquidity")
		}

		amountIn = new(big.Int).Div(numerator, denominator)
		amountIn.Add(amountIn, big.NewInt(1)) // Round up
	}

	return amountIn, nil
}

// pairABIJson is the ABI for the pair contract
const pairABIJson = `[{"constant":true,"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint112","name":"_reserve0","type":"uint112"},{"internalType":"uint112","name":"_reserve1","type":"uint112"},{"internalType":"uint32","name":"_blockTimestampLast","type":"uint32"}],"payable":false,"stateMutability":"view","type":"function"}]`
