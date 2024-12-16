package utils

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
)

const UniswapV2RouterABI = `[{"inputs":[{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactTokensForTokens","outputs":[{"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"stateMutability":"nonpayable","type":"function"}]`

// SwapParams represents parameters for a swap transaction
type SwapParams struct {
	TokenIn      common.Address
	TokenOut     common.Address
	AmountIn     *big.Int
	AmountOutMin *big.Int
	Path         []common.Address
	To           common.Address
	Deadline     *big.Int
}

// TransactionDecoder handles decoding of transaction data
type TransactionDecoder struct {
	uniswapV2Router abi.ABI
	logger          *zap.Logger
}

// NewTransactionDecoder creates a new transaction decoder
func NewTransactionDecoder(logger *zap.Logger) (*TransactionDecoder, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	v2Router, err := abi.JSON(strings.NewReader(UniswapV2RouterABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse UniswapV2Router ABI: %w", err)
	}

	return &TransactionDecoder{
		uniswapV2Router: v2Router,
		logger:          logger,
	}, nil
}

// DecodeSwapParams decodes swap parameters from transaction data
func DecodeSwapParams(data []byte, logger *zap.Logger) (*SwapParams, error) {
	decoder, err := NewTransactionDecoder(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	return decoder.DecodeSwap(data)
}

// DecodeSwap decodes a swap transaction
func (d *TransactionDecoder) DecodeSwap(data []byte) (*SwapParams, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("invalid data length")
	}

	// Decode method signature
	method, err := d.uniswapV2Router.MethodById(data[:4])
	if err != nil {
		return nil, fmt.Errorf("failed to decode method: %w", err)
	}

	// Decode parameters
	params := make(map[string]interface{})
	err = method.Inputs.UnpackIntoMap(params, data[4:])
	if err != nil {
		return nil, fmt.Errorf("failed to decode parameters: %w", err)
	}

	// Extract path
	path, ok := params["path"].([]common.Address)
	if !ok || len(path) < 2 {
		return nil, fmt.Errorf("invalid path")
	}

	// Extract amounts
	amountIn, ok := params["amountIn"].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid amountIn")
	}

	amountOutMin, ok := params["amountOutMin"].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid amountOutMin")
	}

	// Extract to address
	to, ok := params["to"].(common.Address)
	if !ok {
		return nil, fmt.Errorf("invalid to address")
	}

	// Extract deadline
	deadline, ok := params["deadline"].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid deadline")
	}

	return &SwapParams{
		TokenIn:      path[0],
		TokenOut:     path[len(path)-1],
		AmountIn:     amountIn,
		AmountOutMin: amountOutMin,
		Path:         path,
		To:           to,
		Deadline:     deadline,
	}, nil
}

// EncodeSwapExactTokensForTokens encodes parameters for swapExactTokensForTokens
func (d *TransactionDecoder) EncodeSwapExactTokensForTokens(params *SwapParams) ([]byte, error) {
	if params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}

	return d.uniswapV2Router.Pack(
		"swapExactTokensForTokens",
		params.AmountIn,
		params.AmountOutMin,
		params.Path,
		params.To,
		params.Deadline,
	)
}
