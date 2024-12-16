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
		log.Fatal("Error loading .env file:", err)
	}

	// Load secure config
	secureConfig, err := config.LoadSecureConfig()
	assert.NoError(t, err)
	assert.NotEmpty(t, secureConfig.PrivateKey)
	assert.NotEmpty(t, secureConfig.FlashbotsKey)

	// Load main config
	cfg, err := config.LoadConfig("")
	assert.NoError(t, err)
	assert.Equal(t, uint64(11155111), cfg.ChainID) // Sepolia chain ID
	assert.NotEmpty(t, cfg.RPCEndpoint)
	assert.NotEmpty(t, cfg.WSEndpoint)
	assert.True(t, cfg.CircuitBreaker.Enabled)
	assert.NotNil(t, cfg.Logger)
}
