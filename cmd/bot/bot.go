package bot

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"

	"github.com/michaelpento.lv/mevbot/config"
	"github.com/michaelpento.lv/mevbot/mempool"
)

// OpportunityType represents different types of MEV opportunities
type OpportunityType int

const (
	// OpportunityArbitrage represents arbitrage opportunities
	OpportunityArbitrage OpportunityType = iota
	// OpportunityLiquidation represents liquidation opportunities
	OpportunityLiquidation
	// OpportunitySandwich represents sandwich trading opportunities
	OpportunitySandwich
)

// Opportunity represents an MEV opportunity
type Opportunity struct {
	Type    OpportunityType
	Tx      *types.Transaction
	Profit  *big.Int
	GasCost *big.Int
	Target  common.Address
}

// Bot represents the MEV bot instance
type Bot struct {
	cfg            *config.Config
	mempoolMonitor *mempool.MempoolMonitor
	analyzer       *mempool.Analyzer
	logger         *zap.Logger
	wg             sync.WaitGroup
	opportunities  chan *mempool.Opportunity
}

// New creates a new MEV bot instance
func New(cfg *config.Config, logger *zap.Logger) (*Bot, error) {
	// Create Ethereum client
	client, err := ethclient.Dial(cfg.Network.RPCEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}

	// Create mempool monitor
	monitor, err := mempool.NewMempoolMonitor(cfg, client, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create mempool monitor: %w", err)
	}

	// Create analyzer config
	analyzerCfg := mempool.Config{
		Network: struct {
			HTTPEndpoint string
			WSEndpoint   string
		}{
			HTTPEndpoint: cfg.Network.RPCEndpoint,
			WSEndpoint:   cfg.Network.WSEndpoint,
		},
	}

	// Create transaction analyzer
	analyzer, err := mempool.NewAnalyzer(analyzerCfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create analyzer: %w", err)
	}

	return &Bot{
		cfg:            cfg,
		mempoolMonitor: monitor,
		analyzer:       analyzer,
		logger:         logger,
		opportunities:  make(chan *mempool.Opportunity, 100),
	}, nil
}

// Start starts the MEV bot
func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("Starting MEV bot...")

	// Start mempool monitoring
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		txChan := b.mempoolMonitor.Start(ctx)
		for tx := range txChan {
			// Analyze transaction for opportunities
			opportunities := b.analyzer.AnalyzeTransaction(ctx, tx)

			// Send opportunities to processing channel
			for _, opp := range opportunities {
				select {
				case <-ctx.Done():
					return
				case b.opportunities <- opp:
				}
			}
		}
	}()

	// Start opportunity processing
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.processOpportunities(ctx)
	}()

	return nil
}

// Stop stops the MEV bot
func (b *Bot) Stop() {
	b.logger.Info("Stopping MEV bot...")
	close(b.opportunities)
	b.wg.Wait()
}

// processOpportunities processes MEV opportunities
func (b *Bot) processOpportunities(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case opp, ok := <-b.opportunities:
			if !ok {
				return
			}
			if err := b.executeOpportunity(ctx, opp); err != nil {
				b.logger.Error("Failed to execute opportunity",
					zap.Error(err),
					zap.Int("type", int(opp.Type)),
					zap.String("profit", opp.Profit.String()))
			}
		}
	}
}

// executeOpportunity executes an MEV opportunity
func (b *Bot) executeOpportunity(ctx context.Context, opp *mempool.Opportunity) error {
	// Log opportunity details
	b.logger.Info("Executing opportunity",
		zap.Int("type", int(opp.Type)),
		zap.String("profit", opp.Profit.String()),
		zap.String("gas_cost", opp.GasCost.String()),
		zap.Float64("priority", opp.Priority))

	// TODO: Implement opportunity execution
	// This will be expanded in Phase 3 with actual execution logic
	return nil
}
