package mempool

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EthClient defines the interface for Ethereum client operations
type EthClient interface {
	ChainID(ctx context.Context) (*big.Int, error)
	BlockNumber(ctx context.Context) (uint64, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	TransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error)
	SubscribePendingTransactions(ctx context.Context, ch chan<- common.Hash) (ethereum.Subscription, error)
}

// EthClientWrapper wraps ethclient.Client to implement EthClient interface
type EthClientWrapper struct {
	*ethclient.Client
}

// NewEthClientWrapper creates a new EthClientWrapper
func NewEthClientWrapper(client *ethclient.Client) *EthClientWrapper {
	return &EthClientWrapper{Client: client}
}

// SubscribePendingTransactions implements the EthClient interface
func (w *EthClientWrapper) SubscribePendingTransactions(ctx context.Context, ch chan<- common.Hash) (ethereum.Subscription, error) {
	return w.Client.SubscribePendingTransactions(ctx, ch)
}
