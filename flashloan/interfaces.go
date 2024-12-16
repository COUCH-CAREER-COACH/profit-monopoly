package flashloan

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Provider defines the interface for flash loan providers
type Provider interface {
	ExecuteFlashLoan(ctx context.Context, params FlashLoanParams) (*types.Transaction, error)
	GetFlashLoanFee(ctx context.Context, token common.Address) (*big.Int, error)
	GetLiquidity(ctx context.Context, token common.Address) (*big.Int, error)
	String() string
}

// FlashLoanParams contains parameters for executing a flash loan
type FlashLoanParams struct {
	Token  common.Address
	Amount *big.Int
	Data   []byte
}

// ProviderType represents different flash loan providers
type ProviderType int

const (
	ProviderAave ProviderType = iota
	ProviderBalancer
	ProviderDyDx
)
