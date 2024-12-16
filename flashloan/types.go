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

// ProviderConfig contains configuration for flash loan providers
type ProviderConfig struct {
	ContractAddress   common.Address
	MinLoanAmount    string
	MaxLoanAmount    string
	MaxLoanPercentage uint8
	BaseFee          uint16 // In basis points (1 = 0.01%)
}

// FlashLoanParams contains parameters for executing a flash loan
type FlashLoanParams struct {
	Target        common.Address   // Target contract to receive the flash loan
	Token         common.Address   // Token to borrow
	Amount        *big.Int        // Amount to borrow
	Data          []byte          // Arbitrary data to pass to the target contract
	RepaymentPath []common.Address // Path for repayment
	GasPrice      *big.Int        // Gas price for the transaction
	GasLimit      uint64          // Gas limit for the transaction
}
