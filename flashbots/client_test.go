package flashbots

import (
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlashbotsClient(t *testing.T) {
	// Load environment variables
	err := godotenv.Load("../.env")
	require.NoError(t, err, "Failed to load .env file")

	// Get Flashbots configuration
	relayURL := os.Getenv("FLASHBOTS_RELAY_URL")
	require.NotEmpty(t, relayURL, "FLASHBOTS_RELAY_URL not set")

	authKeyHex := os.Getenv("FLASHBOTS_SIGNER_KEY")
	require.NotEmpty(t, authKeyHex, "FLASHBOTS_SIGNER_KEY not set")

	// Parse auth key
	authKey, err := crypto.HexToECDSA(authKeyHex)
	require.NoError(t, err, "Failed to parse auth key")

	// Use the same key for bundle signing in test (in production, use the wallet's private key)
	bundleKey := authKey

	// Create Flashbots client
	client := NewClient(relayURL, authKey, bundleKey, big.NewInt(11155111)) // Sepolia chain ID

	// Test GetStats
	t.Run("GetStats", func(t *testing.T) {
		stats, err := client.GetStats(big.NewInt(4372034)) // Recent Sepolia block
		require.NoError(t, err, "Failed to get stats")
		assert.NotNil(t, stats, "Stats should not be nil")

		// Log the stats for inspection
		t.Logf("Flashbots Stats: %+v", stats)
	})

	// Test creating an empty bundle (just to verify the request format)
	t.Run("SendEmptyBundle", func(t *testing.T) {
		bundle := &Bundle{
			Txs:         [][]byte{},
			BlockNumber: big.NewInt(4372034), // Target next block
		}

		err := client.SendBundle(bundle)
		// It's expected to fail with an empty bundle, we just want to verify the request format
		assert.Error(t, err, "Empty bundle should be rejected")
		t.Logf("Expected error for empty bundle: %v", err)
	})

	// Get the auth signer's address
	authAddress := crypto.PubkeyToAddress(authKey.PublicKey)
	t.Logf("Auth Signer Address: %s", authAddress.Hex())
}
