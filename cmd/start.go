package cmd

import (
	"context"
	"github.com/michaelpento.lv/mevbot/config"
	"github.com/michaelpento.lv/mevbot/mempool"
	"github.com/michaelpento.lv/mevbot/strategies/frontrun"
	"github.com/michaelpento.lv/mevbot/strategies/sandwich"
	"github.com/michaelpento.lv/mevbot/utils"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the MEV bot",
	Run: func(cmd *cobra.Command, args []string) {
		log := utils.GetLogger()

		// Create new configuration
		cfg, err := config.NewConfig()
		if err != nil {
			log.Fatal("Failed to load config", zap.Error(err))
		}

		// Create mempool monitor
		ethClient, err := ethclient.Dial(cfg.RPCEndpoint)
		if err != nil {
			log.Fatal("Failed to connect to Ethereum node", zap.Error(err))
		}

		// Wrap ethClient to implement EthClient interface
		wrappedClient := mempool.NewEthClientWrapper(ethClient)

		monitor, err := mempool.NewMempoolMonitor(cfg, wrappedClient, log)
		if err != nil {
			log.Fatal("Failed to create mempool monitor", zap.Error(err))
		}
		defer monitor.Cleanup()

		// Initialize strategies
		sandwichAttack, err := sandwich.NewSandwichAttack(cfg)
		if err != nil {
			log.Fatal("Failed to initialize sandwich attack strategy", zap.Error(err))
		}
		frontrunStrategy, err := frontrun.NewStrategy(context.Background(), log, &frontrun.Config{})
		if err != nil {
			log.Fatal("Failed to create frontrun strategy", zap.Error(err))
		}

		// Create context with cancellation
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start monitoring
		txChan := monitor.Start(ctx)

		// Process transactions
		for tx := range txChan {
			select {
			case <-ctx.Done():
				return
			default:
				// Check for sandwich opportunities
				if sandwichAttack.IsProfitable(tx) {
					go func(tx *types.Transaction) {
						if err := sandwichAttack.Execute(ctx, tx); err != nil {
							log.Error("Failed to execute sandwich attack", zap.Error(err))
						}
					}(tx)
				}

				// Check for frontrun opportunities
				if frontrunStrategy.IsProfitable(tx) {
					go func(tx *types.Transaction) {
						if err := frontrunStrategy.Execute(tx); err != nil {
							log.Error("Failed to execute frontrun strategy", zap.Error(err))
						}
					}(tx)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
}