package bot

import (
	"context"
	"fmt"
	"github.com/michaelpento.lv/mevbot/config"
	"github.com/michaelpento.lv/mevbot/flashloan"
	"github.com/michaelpento.lv/mevbot/mempool"
	"sync"

	"go.uber.org/zap"
)

// Bot represents the MEV bot instance
type Bot struct {
	cfg            *config.Config
	mempoolMonitor *mempool.Monitor
	analyzer       *mempool.Analyzer
	flashManager   *flashloan.Manager
	logger         *zap.Logger
	wg             sync.WaitGroup
}

// New creates a new MEV bot instance
func New(cfg *config.Config, logger *zap.Logger) (*Bot, error) {
	// Create mempool monitor
	monitor, err := mempool.NewMonitor(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create mempool monitor: %w", err)
	}

	// Create transaction analyzer
	analyzer := mempool.NewAnalyzer(monitor, logger, cfg.Workers)

	// Create flash loan manager
	flashManager, err := flashloan.NewManager(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create flash loan manager: %w", err)
	}

	return &Bot{
		cfg:            cfg,
		mempoolMonitor: monitor,
		analyzer:       analyzer,
		flashManager:   flashManager,
		logger:         logger,
	}, nil
}

// Start starts the MEV bot
func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("Starting MEV bot...")

	// Start mempool monitoring
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		if err := b.mempoolMonitor.Start(ctx); err != nil {
			b.logger.Error("Mempool monitor error", zap.Error(err))
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
	b.wg.Wait()
}

// processOpportunities processes MEV opportunities
func (b *Bot) processOpportunities(ctx context.Context) {
	// Start the analyzer
	opportunities := b.analyzer.Start(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case opp := <-opportunities:
			if err := b.executeOpportunity(ctx, opp); err != nil {
				b.logger.Error("Failed to execute opportunity",
					zap.Error(err),
					zap.String("tx_hash", opp.Tx.Hash().Hex()),
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
		zap.String("tx_hash", opp.Tx.Hash().Hex()),
		zap.String("profit", opp.Profit.String()),
		zap.String("gas_cost", opp.GasCost.String()))

	// TODO: Implement opportunity execution
	// This will be expanded in Phase 3 with actual execution logic
	return nil
}
