package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}

	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Error getting home directory:", err)
	}

	// Config file path
	cfgFile := filepath.Join(home, ".mevbot.json")

	// Test environment variables
	fmt.Println("Environment Variables:")
	fmt.Printf("Sepolia RPC URL: %s\n", os.Getenv("ETHEREUM_SEPOLIA_RPC_URL"))
	fmt.Printf("Sepolia WS URL: %s\n", os.Getenv("ETHEREUM_SEPOLIA_WS_URL"))
	fmt.Printf("Wallet Address: %s\n", os.Getenv("TESTNET_WALLET_ADDRESS"))
	fmt.Printf("Flashbots Relay: %s\n", os.Getenv("FLASHBOTS_RELAY_URL"))

	// Test config file existence
	if _, err := os.Stat(cfgFile); err == nil {
		fmt.Printf("\nConfig file exists at: %s\n", cfgFile)
	} else {
		fmt.Printf("\nConfig file not found at: %s\n", cfgFile)
	}
}
