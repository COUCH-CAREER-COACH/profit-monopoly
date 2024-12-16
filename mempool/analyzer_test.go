package mempool

import (
	"testing"

	"github.com/michaelpento.lv/mevbot/utils/testutils"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeTransaction(t *testing.T) {
	// Create mock transaction
	mockTx := testutils.CreateMockTransaction(t)
	
	// Create analyzer
	analyzer := NewAnalyzer()
	
	// Analyze transaction
	tx := NewTransaction(mockTx, mockTx.To())
	result := analyzer.AnalyzeTransaction(tx)
	
	// Basic assertions
	assert.NotNil(t, result)
	assert.Equal(t, tx.Hash, result.Transaction.Hash)
}
