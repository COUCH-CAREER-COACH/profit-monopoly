package aave

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/michaelpento.lv/mevbot/flashloan"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockEthClient struct {
	liquidityMap map[common.Address]*big.Int
	nonce       uint64
}

func newMockEthClient() *mockEthClient {
	return &mockEthClient{
		liquidityMap: make(map[common.Address]*big.Int),
		nonce:       0,
	}
}

func (m *mockEthClient) CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	token := msg.Data[4:36] // Extract token address from data
	liquidity := m.liquidityMap[common.BytesToAddress(token)]
	if liquidity == nil {
		liquidity = big.NewInt(1000000000000000000) // Default 1 ETH
	}
	result := make([]byte, 32)
	liquidity.FillBytes(result)
	return result, nil
}

func (m *mockEthClient) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	return m.nonce, nil
}

func TestAaveProvider(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockClient := newMockEthClient()

	config := &flashloan.ProviderConfig{
		Type:              flashloan.ProviderAave,
		ContractAddress:   common.HexToAddress("0x7d2768dE32b0b80b7a3454c06BdAc94A69DDc7A9"),
		MaxLoanPercentage: 75,
		MinLoanAmount:     "1000000000000000000", // 1 ETH
		MaxLoanAmount:     "100000000000000000000", // 100 ETH
		BaseFee:           9, // 0.09%
	}

	provider, err := NewAaveProvider(mockClient, config, logger)
	require.NoError(t, err)
	require.NotNil(t, provider)

	t.Run("GetMaxLoanAmount", func(t *testing.T) {
		token := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
		mockClient.liquidityMap[token] = big.NewInt(200000000000000000000) // 200 ETH

		maxLoan, err := provider.GetMaxLoanAmount(token)
		require.NoError(t, err)
		assert.NotNil(t, maxLoan)
		assert.Equal(t, "100000000000000000000", maxLoan.String()) // Should be capped at 100 ETH
	})

	t.Run("GetLoanFee", func(t *testing.T) {
		token := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
		amount := big.NewInt(10000000000000000000) // 10 ETH

		fee, err := provider.GetLoanFee(token, amount)
		require.NoError(t, err)
		assert.NotNil(t, fee)
		// Fee should be 0.09% of 10 ETH
		expectedFee := big.NewInt(9000000000000000) // 0.009 ETH
		assert.Equal(t, expectedFee.String(), fee.String())
	})

	t.Run("ValidateRepayment", func(t *testing.T) {
		token := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
		tests := []struct {
			name          string
			params        *flashloan.FlashLoanParams
			expectError   bool
			errorContains string
		}{
			{
				name: "valid_params",
				params: &flashloan.FlashLoanParams{
					Token:         token,
					Amount:        big.NewInt(10000000000000000000), // 10 ETH
					Target:        common.HexToAddress("0x1234"),
					GasLimit:      300000,
					GasPrice:      big.NewInt(50000000000),
					RepaymentPath: []common.Address{token},
				},
				expectError: false,
			},
			{
				name: "amount_too_low",
				params: &flashloan.FlashLoanParams{
					Token:         token,
					Amount:        big.NewInt(100000000000000000), // 0.1 ETH
					Target:        common.HexToAddress("0x1234"),
					GasLimit:      300000,
					GasPrice:      big.NewInt(50000000000),
					RepaymentPath: []common.Address{token},
				},
				expectError:   true,
				errorContains: "loan amount below minimum",
			},
			{
				name: "missing_repayment_path",
				params: &flashloan.FlashLoanParams{
					Token:     token,
					Amount:    big.NewInt(10000000000000000000),
					Target:    common.HexToAddress("0x1234"),
					GasLimit:  300000,
					GasPrice:  big.NewInt(50000000000),
				},
				expectError:   true,
				errorContains: "repayment path cannot be empty",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := provider.ValidateRepayment(tt.params)
				if tt.expectError {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.errorContains)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("ExecuteFlashLoan", func(t *testing.T) {
		token := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
		params := &flashloan.FlashLoanParams{
			Token:         token,
			Amount:        big.NewInt(10000000000000000000), // 10 ETH
			Target:        common.HexToAddress("0x1234"),
			GasLimit:      300000,
			GasPrice:      big.NewInt(50000000000),
			RepaymentPath: []common.Address{token},
		}

		tx, err := provider.ExecuteFlashLoan(context.Background(), params)
		require.NoError(t, err)
		require.NotNil(t, tx)

		// Verify metrics were updated
		// Note: In a real test, you'd use a test registry to verify these
	})
}

func BenchmarkAaveProvider(b *testing.B) {
	logger := zaptest.NewLogger(b)
	mockClient := newMockEthClient()
	config := &flashloan.ProviderConfig{
		Type:              flashloan.ProviderAave,
		ContractAddress:   common.HexToAddress("0x7d2768dE32b0b80b7a3454c06BdAc94A69DDc7A9"),
		MaxLoanPercentage: 75,
		MinLoanAmount:     "1000000000000000000",
		MaxLoanAmount:     "100000000000000000000",
		BaseFee:           9,
	}

	provider, _ := NewAaveProvider(mockClient, config, logger)
	token := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")

	b.Run("GetMaxLoanAmount", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			provider.GetMaxLoanAmount(token)
		}
	})

	b.Run("GetLoanFee", func(b *testing.B) {
		amount := big.NewInt(10000000000000000000)
		for i := 0; i < b.N; i++ {
			provider.GetLoanFee(token, amount)
		}
	})

	b.Run("ValidateRepayment", func(b *testing.B) {
		params := &flashloan.FlashLoanParams{
			Token:         token,
			Amount:        big.NewInt(10000000000000000000),
			Target:        common.HexToAddress("0x1234"),
			GasLimit:      300000,
			GasPrice:      big.NewInt(50000000000),
			RepaymentPath: []common.Address{token},
		}
		for i := 0; i < b.N; i++ {
			provider.ValidateRepayment(params)
		}
	})
}
