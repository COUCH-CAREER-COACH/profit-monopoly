package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Environment variables
const (
	EnvInfuraKey        = "INFURA_API_KEY"
	EnvPrivateKey       = "PRIVATE_KEY"
	EnvWalletAddress    = "WALLET_ADDRESS"
	EnvNetwork          = "NETWORK" // mainnet, sepolia, holesky
)

// LoadEnv loads environment variables from .env file
func LoadEnv() error {
	return godotenv.Load()
}

// GetEnvWithDefault gets an environment variable with a default value
func GetEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
