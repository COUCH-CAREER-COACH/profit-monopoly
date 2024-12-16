package aave

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/michaelpento.lv/mevbot/flashloan"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// AaveV2 ABI for flash loan operations
const aaveV2ABI = `[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "receiverAddress",
				"type": "address"
			},
			{
				"internalType": "address[]",
				"name": "assets",
				"type": "address[]"
			},
			{
				"internalType": "uint256[]",
				"name": "amounts",
				"type": "uint256[]"
			},
			{
				"internalType": "bytes",
				"name": "params",
				"type": "bytes"
			}
		],
		"name": "flashLoan",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "asset",
				"type": "address"
			}
		],
		"name": "getReserveData",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "availableLiquidity",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

// AaveProvider implements the flash loan Provider interface for Aave
type AaveProvider struct {
	client    *ethclient.Client
	config    *flashloan.ProviderConfig
	logger    *zap.Logger
	abi       abi.ABI
	mu        sync.RWMutex
	metrics   struct {
		loanCount    prometheus.Counter
		loanVolume   prometheus.Counter
		fees         prometheus.Counter
		latency      prometheus.Histogram
		errors       prometheus.Counter
		poolLiquidity prometheus.GaugeVec
	}
}

// NewAaveProvider creates a new Aave flash loan provider
func NewAaveProvider(client *ethclient.Client, config *flashloan.ProviderConfig, logger *zap.Logger) (*AaveProvider, error) {
	if client == nil {
		return nil, fmt.Errorf("ethclient cannot be nil")
	}
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Parse ABI
	parsedABI, err := abi.JSON(strings.NewReader(aaveV2ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	provider := &AaveProvider{
		client: client,
		config: config,
		logger: logger,
		abi:    parsedABI,
	}

	// Initialize metrics
	provider.metrics.loanCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flashloan_aave_loans_total",
		Help: "Total number of Aave flash loans executed",
	})
	provider.metrics.loanVolume = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flashloan_aave_volume_wei",
		Help: "Total volume of Aave flash loans in wei",
	})
	provider.metrics.fees = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flashloan_aave_fees_wei",
		Help: "Total fees paid for Aave flash loans in wei",
	})
	provider.metrics.latency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "flashloan_aave_latency_seconds",
		Help:    "Latency of Aave flash loan operations",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
	})
	provider.metrics.errors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flashloan_aave_errors_total",
		Help: "Total number of Aave flash loan errors",
	})
	provider.metrics.poolLiquidity = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "flashloan_aave_pool_liquidity_wei",
			Help: "Current liquidity in Aave pools",
		},
		[]string{"token"},
	)

	// Start liquidity monitoring
	go provider.monitorPoolLiquidity()

	return provider, nil
}

// GetMaxLoanAmount returns the maximum amount that can be borrowed
func (p *AaveProvider) GetMaxLoanAmount(token common.Address) (*big.Int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	liquidity, err := p.GetPoolLiquidity(token)
	if err != nil {
		return nil, fmt.Errorf("failed to get pool liquidity: %w", err)
	}

	// Calculate max loan based on config percentage
	maxLoan := new(big.Int).Mul(liquidity, big.NewInt(int64(p.config.MaxLoanPercentage)))
	maxLoan = maxLoan.Div(maxLoan, big.NewInt(100))

	// Ensure it doesn't exceed absolute maximum
	configMax := new(big.Int)
	configMax.SetString(p.config.MaxLoanAmount, 10)
	if maxLoan.Cmp(configMax) > 0 {
		maxLoan = configMax
	}

	return maxLoan, nil
}

// GetLoanFee calculates the fee for a flash loan
func (p *AaveProvider) GetLoanFee(token common.Address, amount *big.Int) (*big.Int, error) {
	if amount == nil || amount.Sign() <= 0 {
		return nil, fmt.Errorf("invalid loan amount")
	}

	// Calculate fee based on base fee percentage
	fee := new(big.Int).Mul(amount, big.NewInt(int64(p.config.BaseFee)))
	fee = fee.Div(fee, big.NewInt(10000)) // Convert basis points to percentage

	return fee, nil
}

// ExecuteFlashLoan executes a flash loan operation
func (p *AaveProvider) ExecuteFlashLoan(ctx context.Context, params *flashloan.FlashLoanParams) (*types.Transaction, error) {
	start := time.Now()
	defer func() {
		p.metrics.latency.Observe(time.Since(start).Seconds())
	}()

	if err := p.ValidateRepayment(params); err != nil {
		p.metrics.errors.Inc()
		return nil, fmt.Errorf("repayment validation failed: %w", err)
	}

	// Prepare flash loan call data
	callData, err := p.abi.Pack("flashLoan",
		params.Target,
		[]common.Address{params.Token},
		[]*big.Int{params.Amount},
		params.Data,
	)
	if err != nil {
		p.metrics.errors.Inc()
		return nil, fmt.Errorf("failed to pack flash loan data: %w", err)
	}

	// Create transaction
	tx, err := p.createFlashLoanTx(ctx, callData, params)
	if err != nil {
		p.metrics.errors.Inc()
		return nil, fmt.Errorf("failed to create flash loan transaction: %w", err)
	}

	// Update metrics
	p.metrics.loanCount.Inc()
	p.metrics.loanVolume.Add(float64(params.Amount.Uint64()))
	if fee, err := p.GetLoanFee(params.Token, params.Amount); err == nil {
		p.metrics.fees.Add(float64(fee.Uint64()))
	}

	return tx, nil
}

// ValidateRepayment validates if repayment is possible
func (p *AaveProvider) ValidateRepayment(params *flashloan.FlashLoanParams) error {
	if params == nil {
		return fmt.Errorf("params cannot be nil")
	}

	// Validate amount
	minAmount := new(big.Int)
	minAmount.SetString(p.config.MinLoanAmount, 10)
	if params.Amount.Cmp(minAmount) < 0 {
		return fmt.Errorf("loan amount below minimum")
	}

	maxAmount, err := p.GetMaxLoanAmount(params.Token)
	if err != nil {
		return fmt.Errorf("failed to get max loan amount: %w", err)
	}
	if params.Amount.Cmp(maxAmount) > 0 {
		return fmt.Errorf("loan amount exceeds maximum")
	}

	// Validate repayment path
	if len(params.RepaymentPath) == 0 {
		return fmt.Errorf("repayment path cannot be empty")
	}

	// Validate gas parameters
	if params.GasPrice == nil || params.GasPrice.Sign() <= 0 {
		return fmt.Errorf("invalid gas price")
	}
	if params.GasLimit == 0 {
		return fmt.Errorf("invalid gas limit")
	}

	return nil
}

// GetPoolLiquidity returns current pool liquidity
func (p *AaveProvider) GetPoolLiquidity(token common.Address) (*big.Int, error) {
	callData, err := p.abi.Pack("getReserveData", token)
	if err != nil {
		return nil, fmt.Errorf("failed to pack getReserveData: %w", err)
	}

	result, err := p.client.CallContract(context.Background(), core.CallMsg{
		To:   &p.config.ContractAddress,
		Data: callData,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get reserve data: %w", err)
	}

	// Parse result
	liquidityBig := new(big.Int).SetBytes(result[:32])
	
	// Update metrics
	p.metrics.poolLiquidity.WithLabelValues(token.Hex()).Set(float64(liquidityBig.Uint64()))

	return liquidityBig, nil
}

// monitorPoolLiquidity periodically monitors pool liquidity
func (p *AaveProvider) monitorPoolLiquidity() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Get list of monitored tokens
		tokens := p.getMonitoredTokens()

		for _, token := range tokens {
			if _, err := p.GetPoolLiquidity(token); err != nil {
				p.logger.Error("Failed to get pool liquidity",
					zap.String("token", token.Hex()),
					zap.Error(err))
			}
		}
	}
}

// createFlashLoanTx creates and signs a flash loan transaction
func (p *AaveProvider) createFlashLoanTx(ctx context.Context, data []byte, params *flashloan.FlashLoanParams) (*types.Transaction, error) {
	// Get nonce
	nonce, err := p.client.PendingNonceAt(ctx, p.config.ContractAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Create transaction
	tx := types.NewTransaction(
		nonce,
		p.config.ContractAddress,
		big.NewInt(0), // No value transfer
		params.GasLimit,
		params.GasPrice,
		data,
	)

	return tx, nil
}

// getMonitoredTokens returns list of tokens to monitor
func (p *AaveProvider) getMonitoredTokens() []common.Address {
	// This should be expanded based on your needs
	return []common.Address{
		common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"), // WETH
		common.HexToAddress("0x6B175474E89094C44Da98b954EedeAC495271d0F"), // DAI
		common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"), // USDC
		common.HexToAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7"), // USDT
	}
}
