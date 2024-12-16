package simulator

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/michaelpento.lv/mevbot/utils/testutils"
	"github.com/stretchr/testify/assert"
)

func TestSimulateTransaction(t *testing.T) {
	sim := NewSimulator()
	
	// Create mock transaction
	mockTx := testutils.CreateMockTransaction(t)
	
	// Simulate transaction
	result, err := sim.SimulateTransaction(mockTx)
	
	// Basic assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestSimulateFlashLoan(t *testing.T) {
	sim := NewSimulator()
	
	// Test data
	loanAmount := "1000000000000000000" // 1 ETH
	tokenAddress := common.HexToAddress("0xETH")
	
	// Simulate flash loan
	result, err := sim.SimulateFlashLoan(tokenAddress, loanAmount)
	
	// Basic assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
}
