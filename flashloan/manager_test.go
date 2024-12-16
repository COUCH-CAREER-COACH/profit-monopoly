package flashloan

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockProvider implements the Provider interface for testing
type mockProvider struct {
	maxLoanAmount *big.Int
	loanFee       *big.Int
	shouldError   bool
	executed      bool
}

func (m *mockProvider) GetMaxLoanAmount(token common.Address) (*big.Int, error) {
	return m.maxLoanAmount, nil
}

func (m *mockProvider) GetLoanFee(token common.Address, amount *big.Int) (*big.Int, error) {
	return m.loanFee, nil
}

func (m *mockProvider) ExecuteFlashLoan(ctx context.Context, params *FlashLoanParams) (*types.Transaction, error) {
	m.executed = true
	if m.shouldError {
		return nil, errors.New("mock error")
	}
	return types.NewTransaction(0, common.Address{}, big.NewInt(0), 0, big.NewInt(0), nil), nil
}

func (m *mockProvider) ValidateRepayment(params *FlashLoanParams) error {
	if m.shouldError {
		return errors.New("mock error")
	}
	return nil
}

func (m *mockProvider) GetPoolLiquidity(token common.Address) (*big.Int, error) {
	return big.NewInt(1000000000000000000), nil
}

func TestFlashLoanManager(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewFlashLoanManager(logger)
	require.NotNil(t, manager)

	t.Run("RegisterProvider", func(t *testing.T) {
		provider := &mockProvider{
			maxLoanAmount: big.NewInt(100000000000000000000),
			loanFee:       big.NewInt(1000000000000000),
		}

		err := manager.RegisterProvider(ProviderAave, provider)
		require.NoError(t, err)

		// Try registering same provider type again
		err = manager.RegisterProvider(ProviderAave, provider)
		require.Error(t, err)
	})

	t.Run("GetBestLoanRate", func(t *testing.T) {
		// Register providers with different fees
		provider1 := &mockProvider{
			maxLoanAmount: big.NewInt(100000000000000000000),
			loanFee:       big.NewInt(2000000000000000),
		}
		provider2 := &mockProvider{
			maxLoanAmount: big.NewInt(100000000000000000000),
			loanFee:       big.NewInt(1000000000000000),
		}

		manager.RegisterProvider(ProviderDyDx, provider1)
		manager.RegisterProvider(ProviderBalancer, provider2)

		token := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
		amount := big.NewInt(10000000000000000000)

		bestProvider, bestFee, err := manager.GetBestLoanRate(token, amount)
		require.NoError(t, err)
		require.NotNil(t, bestProvider)
		require.Equal(t, provider2.loanFee.String(), bestFee.String())
	})

	t.Run("ExecuteFlashLoan", func(t *testing.T) {
		token := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
		params := &FlashLoanParams{
			Token:         token,
			Amount:        big.NewInt(10000000000000000000),
			Target:        common.HexToAddress("0x1234"),
			GasLimit:      300000,
			GasPrice:      big.NewInt(50000000000),
			RepaymentPath: []common.Address{token},
		}

		// Test successful execution
		tx, err := manager.ExecuteFlashLoan(context.Background(), params)
		require.NoError(t, err)
		require.NotNil(t, tx)

		// Test execution with error
		for _, provider := range manager.providers {
			if mp, ok := provider.(*mockProvider); ok {
				mp.shouldError = true
			}
		}
		_, err = manager.ExecuteFlashLoan(context.Background(), params)
		require.Error(t, err)
	})

	t.Run("ValidateFlashLoan", func(t *testing.T) {
		token := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
		params := &FlashLoanParams{
			Token:         token,
			Amount:        big.NewInt(10000000000000000000),
			Target:        common.HexToAddress("0x1234"),
			GasLimit:      300000,
			GasPrice:      big.NewInt(50000000000),
			RepaymentPath: []common.Address{token},
		}

		// Test successful validation
		for _, provider := range manager.providers {
			if mp, ok := provider.(*mockProvider); ok {
				mp.shouldError = false
			}
		}
		err := manager.ValidateFlashLoan(params)
		require.NoError(t, err)

		// Test failed validation
		for _, provider := range manager.providers {
			if mp, ok := provider.(*mockProvider); ok {
				mp.shouldError = true
			}
		}
		err = manager.ValidateFlashLoan(params)
		require.Error(t, err)
	})
}

func BenchmarkFlashLoanManager(b *testing.B) {
	logger := zaptest.NewLogger(b)
	manager := NewFlashLoanManager(logger)

	provider := &mockProvider{
		maxLoanAmount: big.NewInt(100000000000000000000),
		loanFee:       big.NewInt(1000000000000000),
	}
	manager.RegisterProvider(ProviderAave, provider)

	token := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	amount := big.NewInt(10000000000000000000)

	b.Run("GetBestLoanRate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			manager.GetBestLoanRate(token, amount)
		}
	})

	params := &FlashLoanParams{
		Token:         token,
		Amount:        amount,
		Target:        common.HexToAddress("0x1234"),
		GasLimit:      300000,
		GasPrice:      big.NewInt(50000000000),
		RepaymentPath: []common.Address{token},
	}

	b.Run("ValidateFlashLoan", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			manager.ValidateFlashLoan(params)
		}
	})

	b.Run("ExecuteFlashLoan", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			manager.ExecuteFlashLoan(ctx, params)
		}
	})
}
