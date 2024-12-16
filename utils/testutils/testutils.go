package testutils

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// CreateMockTransaction creates a mock transaction for testing
func CreateMockTransaction(t *testing.T) *types.Transaction {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	
	signer := types.NewEIP155Signer(big.NewInt(1))
	
	tx := types.NewTransaction(
		0,                                                      // nonce
		common.HexToAddress("0x1234567890123456789012345678901234567890"), // to
		big.NewInt(1000000000000000000),                       // value (1 ETH)
		21000,                                                 // gas limit
		big.NewInt(20000000000),                              // gas price
		nil,                                                   // data
	)
	
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)
	
	return signedTx
}

func createTestPrivateKey(t *testing.T) []byte {
	key := make([]byte, 32)
	for i := 0; i < 32; i++ {
		key[i] = byte(i)
	}
	return key
}
