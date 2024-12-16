package main

import (
	"context"
	"github.com/michaelpento.lv/mevbot/cmd/bot"
	"github.com/michaelpento.lv/mevbot/config"
	"github.com/michaelpento.lv/mevbot/utils"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	utils.InitLogger(true)
	log := utils.GetLogger()
	defer log.Sync()

	// Load configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Create and start bot
	bot, err := bot.New(cfg, log)
	if err != nil {
		log.Fatal("Failed to create bot", zap.Error(err))
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the bot
	if err := bot.Start(ctx); err != nil {
		log.Fatal("Failed to start bot", zap.Error(err))
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Shutting down gracefully...")
		cancel()
		bot.Stop()
	}()
}
