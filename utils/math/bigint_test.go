package math

import (
	"math/big"
	"reflect"
	"testing"
)

func TestBigInt(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{"TestNewBigInt", testNewBigInt},
		{"TestAdd", testAdd},
		{"TestSub", testSub},
		{"TestMul", testMul},
		{"TestDiv", testDiv},
		{"TestCmp", testCmp},
		{"TestClone", testClone},
		{"TestPriceImpact", testPriceImpact},
		{"TestOptimalAmount", testOptimalAmount},
		{"TestProfitability", testProfitability},
		{"TestSIMDComparison", testSIMDComparison},
		{"TestArbitrageProfitable", testArbitrageProfitable},
		{"TestFlashLoanFee", testFlashLoanFee},
		{"TestAtomicOperations", testAtomicOperations},
		{"TestSandwichAttack", testSandwichAttack},
		{"TestStrategyMetrics", testStrategyMetrics},
		{"TestCrossExchangeArbitrage", testCrossExchangeArbitrage},
		{"TestJustInTimeLiquidation", testJustInTimeLiquidation},
		{"TestOptimizeMEVBundle", testOptimizeMEVBundle},
		{"TestAssessReorgRisk", testAssessReorgRisk},
		{"TestCalculateMultiHopArbitrage", testCalculateMultiHopArbitrage},
		{"TestAnalyzeLendingProtocol", testAnalyzeLendingProtocol},
		{"TestCalculateNFTMEV", testCalculateNFTMEV},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func testNewBigInt(t *testing.T) {
	x := NewBigInt(123)
	if x.Int64() != 123 {
		t.Errorf("NewBigInt(123) = %v; want 123", x.Int64())
	}
}

func testAdd(t *testing.T) {
	x := NewBigInt(100)
	y := NewBigInt(50)
	z := NewBigInt(0)
	z.Add(x, y)
	if z.Int64() != 150 {
		t.Errorf("Add(100, 50) = %v; want 150", z.Int64())
	}
}

func testSub(t *testing.T) {
	x := NewBigInt(100)
	y := NewBigInt(50)
	z := NewBigInt(0)
	z.Sub(x, y)
	if z.Int64() != 50 {
		t.Errorf("Sub(100, 50) = %v; want 50", z.Int64())
	}
}

func testMul(t *testing.T) {
	x := NewBigInt(100)
	y := NewBigInt(50)
	z := NewBigInt(0)
	z.Mul(x, y)
	if z.Int64() != 5000 {
		t.Errorf("Mul(100, 50) = %v; want 5000", z.Int64())
	}
}

func testDiv(t *testing.T) {
	x := NewBigInt(100)
	y := NewBigInt(50)
	z := NewBigInt(0)
	z.Div(x, y)
	if z.Int64() != 2 {
		t.Errorf("Div(100, 50) = %v; want 2", z.Int64())
	}
}

func testCmp(t *testing.T) {
	tests := []struct {
		x, y int64
		want int
	}{
		{100, 50, 1},
		{50, 100, -1},
		{100, 100, 0},
	}

	for _, tt := range tests {
		x := NewBigInt(tt.x)
		y := NewBigInt(tt.y)
		got := x.Cmp(y)
		if got != tt.want {
			t.Errorf("Cmp(%v, %v) = %v; want %v", tt.x, tt.y, got, tt.want)
		}
	}
}

func testClone(t *testing.T) {
	x := NewBigInt(100)
	y := x.Clone()

	// Modify x
	x.Add(x, NewBigInt(50))

	// y should remain unchanged
	if y.Int64() != 100 {
		t.Errorf("Clone() not independent, got %v; want 100", y.Int64())
	}
}

func testPriceImpact(t *testing.T) {
	amount := NewBigInt(1000000)     // 1M tokens
	poolDepth := NewBigInt(10000000) // 10M tokens

	impact := amount.CalculatePriceImpact(poolDepth)
	expectedInt, ok := new(big.Int).SetString("10000000000000000000", 10)
	if !ok {
		t.Fatal("Failed to parse expected value")
	}
	expected := &BigInt{
		Int:         expectedInt,
		atomicCache: 0,
	}

	if impact.Cmp(expected) != 0 {
		t.Errorf("CalculatePriceImpact() = %v; want %v", impact, expected)
	}
}

func testOptimalAmount(t *testing.T) {
	balance := NewBigInt(1000000000) // Much larger balance
	gasPrice := NewBigInt(50)
	slippageTolerance := NewBigInt(1) // 1%
	minProfit := NewBigInt(1000)

	optimal := balance.CalculateOptimalAmount(gasPrice, slippageTolerance, minProfit)
	if optimal.IsZero() {
		t.Error("CalculateOptimalAmount() returned zero for valid input")
	}
}

func testProfitability(t *testing.T) {
	amount := NewBigInt(1000000)
	buyPrice := NewBigInt(100)
	sellPrice := NewBigInt(110)
	gasPrice := NewBigInt(50)
	poolDepth := NewBigInt(10000000)

	profit := amount.EstimateProfitability(buyPrice, sellPrice, gasPrice, poolDepth)
	if profit.IsZero() {
		t.Error("EstimateProfitability() returned zero profit for profitable trade")
	}
}

func testSIMDComparison(t *testing.T) {
	a := []*BigInt{
		NewBigInt(100),
		NewBigInt(200),
		NewBigInt(300),
		NewBigInt(400),
	}
	b := []*BigInt{
		NewBigInt(100),
		NewBigInt(150),
		NewBigInt(300),
		NewBigInt(500),
	}

	expected := []int{0, 1, 0, -1}
	results := CompareArraysSIMD(a, b)

	for i, v := range results {
		if v != expected[i] {
			t.Errorf("CompareArraysSIMD() at index %d = %v; want %v", i, v, expected[i])
		}
	}
}

func testArbitrageProfitable(t *testing.T) {
	// Use much larger numbers to account for gas costs
	profit := NewBigInt(1000000000)
	gasPrice := NewBigInt(50)
	minProfit := NewBigInt(1000)

	if !profit.IsArbitrageProfitable(gasPrice, minProfit) {
		t.Error("IsArbitrageProfitable() returned false for profitable trade")
	}

	smallProfit := NewBigInt(100)
	if smallProfit.IsArbitrageProfitable(gasPrice, minProfit) {
		t.Error("IsArbitrageProfitable() returned true for unprofitable trade")
	}
}

func testFlashLoanFee(t *testing.T) {
	amount := NewBigInt(1000000)
	feeRate := NewBigInt(9) // 0.09%

	fee := amount.CalculateFlashLoanFee(feeRate)
	expected := NewBigInt(900) // 0.09% of 1000000

	if fee.Cmp(expected) != 0 {
		t.Errorf("CalculateFlashLoanFee() = %v; want %v", fee, expected)
	}
}

func testAtomicOperations(t *testing.T) {
	x := NewBigInt(12345)

	x.AtomicStore(67890)
	loaded := x.AtomicLoad()

	if loaded != 67890 {
		t.Errorf("Atomic operations failed: got %v; want 67890", loaded)
	}
}

func testSandwichAttack(t *testing.T) {
	params := SandwichAttackParams{
		TargetAmount:     NewBigInt(1000000),
		PoolLiquidity:    NewBigInt(10000000),
		FrontrunGasPrice: NewBigInt(100),
		BackrunGasPrice:  NewBigInt(80),
		MaxBlockDelay:    NewBigInt(1),
		CompetitorFactor: NewBigInt(20), // 20% competitor presence
	}

	frontrun, backrun, profit := CalculateSandwichProfitability(params)

	if frontrun.IsZero() {
		t.Error("Expected non-zero frontrun amount")
	}
	if backrun.IsZero() {
		t.Error("Expected non-zero backrun amount")
	}
	if profit.IsZero() {
		t.Error("Expected non-zero profit for valid sandwich opportunity")
	}

	// Test with zero amounts
	zeroParams := SandwichAttackParams{
		TargetAmount:     NewBigInt(0),
		PoolLiquidity:    NewBigInt(10000000),
		FrontrunGasPrice: NewBigInt(100),
		BackrunGasPrice:  NewBigInt(80),
		MaxBlockDelay:    NewBigInt(1),
		CompetitorFactor: NewBigInt(20),
	}

	zeroFrontrun, zeroBackrun, zeroProfit := CalculateSandwichProfitability(zeroParams)
	if !zeroFrontrun.IsZero() || !zeroBackrun.IsZero() || !zeroProfit.IsZero() {
		t.Error("Expected zero results for zero target amount")
	}
}

func testStrategyMetrics(t *testing.T) {
	metrics := NewStrategyMetrics()

	// Test successful trade
	metrics.UpdateMetrics(
		true,
		NewBigInt(1000),
		NewBigInt(100),
		false,
		NewBigInt(10),
	)

	if metrics.SuccessRate.IsZero() {
		t.Error("Expected non-zero success rate")
	}
	if metrics.AverageProfit.IsZero() {
		t.Error("Expected non-zero average profit")
	}
	if !metrics.CompetitorWins.IsZero() {
		t.Error("Expected zero competitor wins")
	}

	// Test failed trade
	metrics.UpdateMetrics(
		false,
		NewBigInt(0),
		NewBigInt(100),
		true,
		NewBigInt(11),
	)

	if metrics.FailureRate.IsZero() {
		t.Error("Expected non-zero failure rate")
	}
	if metrics.CompetitorWins.IsZero() {
		t.Error("Expected non-zero competitor wins")
	}
}

func testCrossExchangeArbitrage(t *testing.T) {
	amountIn := NewBigInt(1000000)
	sourcePrice := NewBigInt(100)
	targetPrice := NewBigInt(110)
	sourceLiquidity := NewBigInt(10000000)
	targetLiquidity := NewBigInt(10000000)
	gasPrice := NewBigInt(50)

	profit := CrossExchangeArbitrage(
		amountIn,
		sourcePrice,
		targetPrice,
		sourceLiquidity,
		targetLiquidity,
		gasPrice,
	)

	if profit.IsZero() {
		t.Error("Expected non-zero profit for valid arbitrage opportunity")
	}

	// Test with zero amount
	zeroProfit := CrossExchangeArbitrage(
		NewBigInt(0),
		sourcePrice,
		targetPrice,
		sourceLiquidity,
		targetLiquidity,
		gasPrice,
	)

	if !zeroProfit.IsZero() {
		t.Error("Expected zero profit for zero input amount")
	}
}

func testJustInTimeLiquidation(t *testing.T) {
	debtAmount := NewBigInt(1000000)
	collateralAmount := NewBigInt(1500000)
	collateralPrice := NewBigInt(1)
	liquidationBonus := NewBigInt(500) // 5%
	gasPrice := NewBigInt(50)

	bidAmount, profit := JustInTimeLiquidation(
		debtAmount,
		collateralAmount,
		collateralPrice,
		liquidationBonus,
		gasPrice,
	)

	if bidAmount.Cmp(debtAmount) <= 0 {
		t.Error("Expected bid amount to be greater than debt amount")
	}
	if profit.IsZero() {
		t.Error("Expected non-zero profit for valid liquidation opportunity")
	}

	// Test with zero amounts
	zeroBid, zeroProfit := JustInTimeLiquidation(
		NewBigInt(0),
		collateralAmount,
		collateralPrice,
		liquidationBonus,
		gasPrice,
	)

	if !zeroBid.IsZero() || !zeroProfit.IsZero() {
		t.Error("Expected zero results for zero debt amount")
	}
}

func testOptimizeMEVBundle(t *testing.T) {
	tests := []struct {
		name        string
		params      BundleParams
		wantProfit  int64
		wantGasUsed int64
	}{
		{
			name: "basic_bundle",
			params: BundleParams{
				BlockBuilderFee: NewBigInt(100),
				Priority:        NewBigInt(200),
				GasLimit:        NewBigInt(21000),
				RevertProtected: false,
			},
			wantProfit:  -2080000, // (200 * 100) - (21000 * 100)
			wantGasUsed: 21000,
		},
		{
			name: "with_revert_protection",
			params: BundleParams{
				BlockBuilderFee: NewBigInt(100),
				Priority:        NewBigInt(200),
				GasLimit:        NewBigInt(21000),
				RevertProtected: true,
			},
			wantProfit:  -2290000, // (200 * 100) - (21000 * 100 * 1.1)
			wantGasUsed: 21000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProfit, gotGasUsed := OptimizeMEVBundle(tt.params)
			if gotProfit.Int64() != tt.wantProfit {
				t.Errorf("OptimizeMEVBundle() profit = %v, want %v", gotProfit, tt.wantProfit)
			}
			if gotGasUsed.Int64() != tt.wantGasUsed {
				t.Errorf("OptimizeMEVBundle() gasUsed = %v, want %v", gotGasUsed, tt.wantGasUsed)
			}
		})
	}
}

func testAssessReorgRisk(t *testing.T) {
	tests := []struct {
		name   string
		params ReorgRiskParams
		want   int64
	}{
		{
			name: "low_risk",
			params: ReorgRiskParams{
				BlockDepth:      NewBigInt(1),
				HashPower:       NewBigInt(30),  // 30% hash power
				NetworkLatency:  NewBigInt(100), // 100ms
				CompetitorCount: NewBigInt(2),   // 2 competitors
			},
			want: 26, // 30 * 0.9 * 1.1 * 1.08 = 26
		},
		{
			name: "high_risk",
			params: ReorgRiskParams{
				BlockDepth:      NewBigInt(1),
				HashPower:       NewBigInt(50),  // 50% hash power
				NetworkLatency:  NewBigInt(200), // 200ms
				CompetitorCount: NewBigInt(5),   // 5 competitors
			},
			want: 44, // 50 * 0.9 * 1.05 * 1.20 = 44
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessReorgRisk(tt.params)
			if got.Int64() != tt.want {
				t.Errorf("AssessReorgRisk() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testCalculateMultiHopArbitrage(t *testing.T) {
	tests := []struct {
		name       string
		params     MultiHopParams
		wantPath   []int
		wantProfit int64
	}{
		{
			name: "profitable_path",
			params: MultiHopParams{
				Amounts:      []*BigInt{NewBigInt(1000), NewBigInt(1000), NewBigInt(1000)},
				Prices:       []*BigInt{NewBigInt(100), NewBigInt(102), NewBigInt(103)},
				Liquidities:  []*BigInt{NewBigInt(10000), NewBigInt(10000), NewBigInt(10000)},
				GasCosts:     []*BigInt{NewBigInt(10), NewBigInt(10), NewBigInt(10)},
				MaxHops:      2,
				MinProfitBPS: NewBigInt(50), // 0.5%
			},
			wantPath:   []int{0, 2},
			wantProfit: 102990, // (1000 * 103) - 10
		},
		{
			name: "unprofitable_path",
			params: MultiHopParams{
				Amounts:      []*BigInt{NewBigInt(1000), NewBigInt(1000)},
				Prices:       []*BigInt{NewBigInt(100), NewBigInt(99)},
				Liquidities:  []*BigInt{NewBigInt(10000), NewBigInt(10000)},
				GasCosts:     []*BigInt{NewBigInt(1000), NewBigInt(1000)},
				MaxHops:      2,
				MinProfitBPS: NewBigInt(1000), // 10%
			},
			wantPath:   nil,
			wantProfit: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotProfit := CalculateMultiHopArbitrage(tt.params)
			if !reflect.DeepEqual(gotPath, tt.wantPath) {
				t.Errorf("CalculateMultiHopArbitrage() path = %v, want %v", gotPath, tt.wantPath)
			}
			if gotProfit.Int64() != tt.wantProfit {
				t.Errorf("CalculateMultiHopArbitrage() profit = %v, want %v", gotProfit, tt.wantProfit)
			}
		})
	}
}

func testAnalyzeLendingProtocol(t *testing.T) {
	tests := []struct {
		name             string
		params           LendingProtocolParams
		wantShouldBorrow bool
		wantAmount       int64
	}{
		{
			name: "profitable_borrow",
			params: LendingProtocolParams{
				BorrowRate:           NewBigInt(500),  // 5%
				SupplyRate:           NewBigInt(800),  // 8%
				Utilization:          NewBigInt(8000), // 80%
				CollateralValue:      NewBigInt(10000),
				DebtValue:            NewBigInt(5000),
				LiquidationThreshold: NewBigInt(8000), // 80%
			},
			wantShouldBorrow: true,
			wantAmount:       2400,
		},
		{
			name: "unprofitable_borrow",
			params: LendingProtocolParams{
				BorrowRate:           NewBigInt(800),
				SupplyRate:           NewBigInt(500),
				Utilization:          NewBigInt(9000),
				CollateralValue:      NewBigInt(10000),
				DebtValue:            NewBigInt(7000),
				LiquidationThreshold: NewBigInt(8000),
			},
			wantShouldBorrow: false,
			wantAmount:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotShouldBorrow, gotAmount := AnalyzeLendingProtocol(tt.params)
			if gotShouldBorrow != tt.wantShouldBorrow {
				t.Errorf("AnalyzeLendingProtocol() shouldBorrow = %v, want %v", gotShouldBorrow, tt.wantShouldBorrow)
			}
			if gotAmount.Int64() != tt.wantAmount {
				t.Errorf("AnalyzeLendingProtocol() amount = %v, want %v", gotAmount, tt.wantAmount)
			}
		})
	}
}

func testCalculateNFTMEV(t *testing.T) {
	tests := []struct {
		name           string
		params         NFTMEVParams
		wantProfitable bool
		wantProfit     int64
	}{
		{
			name: "profitable_opportunity",
			params: NFTMEVParams{
				FloorPrice:    NewBigInt(100000), // 1 ETH
				ListingPrice:  NewBigInt(90000),  // 0.9 ETH
				RarityScore:   NewBigInt(120),    // 20% above floor
				HistoricalVol: NewBigInt(10),     // 10% volatility
				GasCost:       NewBigInt(1000),   // 0.01 ETH gas cost
				MinProfitBPS:  NewBigInt(50),     // 0.5% minimum profit
			},
			wantProfitable: true,
			wantProfit:     39000, // (100000 * 1.2) + (100000 * 0.1) - 90000 - 1000 = 39000
		},
		{
			name: "unprofitable_opportunity",
			params: NFTMEVParams{
				FloorPrice:    NewBigInt(100000),
				ListingPrice:  NewBigInt(99000),
				RarityScore:   NewBigInt(105),
				HistoricalVol: NewBigInt(5),
				GasCost:       NewBigInt(2000),
				MinProfitBPS:  NewBigInt(1000), // 10% minimum profit
			},
			wantProfitable: false,
			wantProfit:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProfitable, gotProfit := CalculateNFTMEV(tt.params)
			if gotProfitable != tt.wantProfitable {
				t.Errorf("CalculateNFTMEV() profitable = %v, want %v", gotProfitable, tt.wantProfitable)
			}
			if gotProfit.Int64() != tt.wantProfit {
				t.Errorf("CalculateNFTMEV() profit = %v, want %v", gotProfit, tt.wantProfit)
			}
		})
	}
}

func BenchmarkBigIntOperations(b *testing.B) {
	x := NewBigInt(123456789)
	y := NewBigInt(987654321)
	z := NewBigInt(0)

	b.Run("Add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			z.Add(x, y)
		}
	})

	b.Run("Mul", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			z.Mul(x, y)
		}
	})
}

func BenchmarkMEVOperations(b *testing.B) {
	amount := NewBigInt(1000000)
	poolDepth := NewBigInt(10000000)
	buyPrice := NewBigInt(100)
	sellPrice := NewBigInt(110)
	gasPrice := NewBigInt(50)

	b.Run("PriceImpact", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			amount.CalculatePriceImpact(poolDepth)
		}
	})

	b.Run("Profitability", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			amount.EstimateProfitability(buyPrice, sellPrice, gasPrice, poolDepth)
		}
	})

	b.Run("SIMDComparison", func(b *testing.B) {
		const size = 1000
		a := make([]*BigInt, size)
		bb := make([]*BigInt, size)
		for i := 0; i < size; i++ {
			a[i] = NewBigInt(int64(i))
			bb[i] = NewBigInt(int64(i))
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			CompareArraysSIMD(a, bb)
		}
	})

	b.Run("SandwichAttack", func(b *testing.B) {
		params := SandwichAttackParams{
			TargetAmount:     NewBigInt(1000000),
			PoolLiquidity:    NewBigInt(10000000),
			FrontrunGasPrice: NewBigInt(100),
			BackrunGasPrice:  NewBigInt(80),
			MaxBlockDelay:    NewBigInt(1),
			CompetitorFactor: NewBigInt(20),
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			CalculateSandwichProfitability(params)
		}
	})

	b.Run("CrossExchangeArbitrage", func(b *testing.B) {
		amountIn := NewBigInt(1000000)
		sourcePrice := NewBigInt(100)
		targetPrice := NewBigInt(110)
		sourceLiquidity := NewBigInt(10000000)
		targetLiquidity := NewBigInt(10000000)
		gasPrice := NewBigInt(50)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			CrossExchangeArbitrage(
				amountIn,
				sourcePrice,
				targetPrice,
				sourceLiquidity,
				targetLiquidity,
				gasPrice,
			)
		}
	})

	b.Run("JustInTimeLiquidation", func(b *testing.B) {
		debtAmount := NewBigInt(1000000)
		collateralAmount := NewBigInt(1500000)
		collateralPrice := NewBigInt(1)
		liquidationBonus := NewBigInt(500)
		gasPrice := NewBigInt(50)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			JustInTimeLiquidation(
				debtAmount,
				collateralAmount,
				collateralPrice,
				liquidationBonus,
				gasPrice,
			)
		}
	})
}
