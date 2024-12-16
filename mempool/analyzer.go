package mempool

import (
	"context"
	"fmt"
	"math/big"
	"github.com/michaelpento.lv/mevbot/dex"
	"github.com/michaelpento.lv/mevbot/dex/uniswap"
	"github.com/michaelpento.lv/mevbot/dex/sushiswap"
	"github.com/michaelpento.lv/mevbot/strategies/arbitrage"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

// OpportunityType represents the type of MEV opportunity
type OpportunityType int

const (
	OpportunityArbitrage OpportunityType = iota
	OpportunityLiquidation
)

// Opportunity represents a potential MEV opportunity
type Opportunity struct {
	Type     OpportunityType
	Tx       *Transaction
	Profit   *big.Int
	GasCost  *big.Int
	Priority float64
}

// Config represents the analyzer configuration
type Config struct {
	Network struct {
		HTTPEndpoint string
		WSEndpoint   string
	}
	Tokens []common.Address
}

// Analyzer analyzes transactions for MEV opportunities
type Analyzer struct {
	exchanges []dex.Exchange
	logger    *zap.Logger
	tokens    []common.Address
	baseToken common.Address
	cfg       Config
	detector  *arbitrage.Detector
	mu        sync.Mutex
}

// NewAnalyzer creates a new transaction analyzer
func NewAnalyzer(cfg Config, logger *zap.Logger) (*Analyzer, error) {
	return &Analyzer{
		logger:    logger,
		tokens:    cfg.Tokens,
		baseToken: common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"), // WETH
		cfg:       cfg,
	}, nil
}

// AnalyzeTransaction analyzes a transaction for MEV opportunities
func (a *Analyzer) AnalyzeTransaction(ctx context.Context, tx *Transaction) []*Opportunity {
	a.mu.Lock()
	defer a.mu.Unlock()

	var opportunities []*Opportunity

	// Initialize exchanges if not already done
	if a.exchanges == nil {
		exchanges, err := a.initExchanges()
		if err != nil {
			a.logger.Error("Failed to initialize exchanges", zap.Error(err))
			return nil
		}
		a.exchanges = exchanges
	}

	// Check for arbitrage opportunities
	if opp := a.checkArbitrage(ctx, tx); opp != nil {
		opportunities = append(opportunities, opp)
	}

	// Check for liquidation opportunities
	if opp := a.checkLiquidation(ctx, tx); opp != nil {
		opportunities = append(opportunities, opp)
	}

	return opportunities
}

// initExchanges initializes supported exchanges
func (a *Analyzer) initExchanges() ([]dex.Exchange, error) {
	client, err := ethclient.Dial(a.cfg.Network.HTTPEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}

	// Initialize Uniswap V2
	uniswapV2, err := uniswap.NewUniswapV2(client)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Uniswap V2: %w", err)
	}

	// Initialize Sushiswap
	sushiswapV2, err := sushiswap.NewSushiswapV2(client)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Sushiswap V2: %w", err)
	}

	// Add exchanges
	exchanges := []dex.Exchange{
		uniswapV2,
		sushiswapV2,
	}

	return exchanges, nil
}

// checkArbitrage checks for arbitrage opportunities
func (a *Analyzer) checkArbitrage(ctx context.Context, tx *Transaction) *Opportunity {
	// Initialize detector if not already done
	if a.detector == nil {
		client, err := ethclient.Dial(a.cfg.Network.HTTPEndpoint)
		if err != nil {
			a.logger.Error("Failed to connect to Ethereum node", zap.Error(err))
			return nil
		}

		a.detector = arbitrage.NewDetector(
			client,
			a.exchanges,
			a.tokens,
			big.NewInt(1e16), // 0.01 ETH minimum profit
			a.logger,
		)
	}

	// Find arbitrage opportunities
	routes, err := a.detector.FindArbitrage(ctx, a.baseToken)
	if err != nil {
		a.logger.Error("Failed to find arbitrage", zap.Error(err))
		return nil
	}

	// Find most profitable route
	var bestRoute *arbitrage.Route
	for _, route := range routes {
		if bestRoute == nil || route.Profit.Cmp(bestRoute.Profit) > 0 {
			bestRoute = route
		}
	}

	if bestRoute != nil {
		return &Opportunity{
			Type:     OpportunityArbitrage,
			Tx:       tx,
			Profit:   bestRoute.Profit,
			GasCost:  bestRoute.GasCost,
			Priority: float64(bestRoute.Profit.Int64()) / float64(bestRoute.GasCost.Int64()),
		}
	}

	return nil
}

// checkLiquidation checks for liquidation opportunities
func (a *Analyzer) checkLiquidation(ctx context.Context, tx *Transaction) *Opportunity {
	// TODO: Implement liquidation detection
	return nil
}
