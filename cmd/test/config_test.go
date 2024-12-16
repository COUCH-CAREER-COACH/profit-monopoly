package main

import (
	"log"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/michaelpento.lv/mevbot/config"
)

func TestConfigLoading(t *testing.T) {
	// Load .env file
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		// Don't fatal here, as env vars might be set another way
	}

	// Load main config first
	cfg, err := config.LoadConfig("")
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Test network configuration
	assert.Equal(t, int64(11155111), cfg.Network.ChainID) // Sepolia chain ID
	assert.NotEmpty(t, cfg.Network.RPCEndpoint)
	assert.NotEmpty(t, cfg.Network.WSEndpoint)

	// Test mempool configuration
	assert.Greater(t, cfg.MempoolConfig.MaxPendingTx, 0)
	assert.Greater(t, cfg.MempoolConfig.BlockBufferSize, 0)
	assert.NotEmpty(t, cfg.MempoolConfig.DataDir)

	// Test flash loan configuration
	assert.NotEmpty(t, cfg.FlashLoan.Providers)
	assert.Greater(t, cfg.FlashLoan.MaxLoanAmount, 0.0)
	assert.Greater(t, cfg.FlashLoan.MinProfitRatio, 0.0)

	// Test monitoring configuration
	assert.Greater(t, cfg.Monitoring.PrometheusPort, 0)
	assert.Greater(t, cfg.Monitoring.HealthCheckPort, 0)
	assert.NotEmpty(t, cfg.Monitoring.LogLevel)
}
