package mempool

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
	"time"
)

// Transaction represents a transaction in the mempool
type Transaction struct {
	Hash      common.Hash
	From      common.Address
	To        *common.Address
	Value     *big.Int
	GasPrice  *big.Int
	GasLimit  uint64
	Nonce     uint64
	Data      []byte
	Timestamp time.Time
	Raw       *types.Transaction
}

// NewTransaction creates a new Transaction from an eth transaction
func NewTransaction(tx *types.Transaction, from common.Address) *Transaction {
	return &Transaction{
		Hash:      tx.Hash(),
		From:      from,
		To:        tx.To(),
		Value:     tx.Value(),
		GasPrice:  tx.GasPrice(),
		GasLimit:  tx.Gas(),
		Nonce:     tx.Nonce(),
		Data:      tx.Data(),
		Timestamp: time.Now(),
		Raw:       tx,
	}
}
