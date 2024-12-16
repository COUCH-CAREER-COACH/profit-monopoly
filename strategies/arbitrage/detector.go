package arbitrage

import (
	"context"
	"fmt"
	"math/big"
	"github.com/michaelpento.lv/mevbot/dex"
	"github.com/michaelpento.lv/mevbot/simulator"
	"github.com/michaelpento.lv/mevbot/types"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

// Route represents an arbitrage route
type Route struct {
	Path           []common.Address
	Exchanges      []dex.Exchange
	InputAmount    *big.Int
	OutputAmount   *big.Int
	Profit         *big.Int
	GasCost        *big.Int
	Simulated      bool
	SimulationGas  uint64
	FlashLoanPool  common.Address
	FlashLoanToken common.Address
}

// Detector handles arbitrage detection
type Detector struct {
	client     *ethclient.Client
	exchanges  []dex.Exchange
	tokens     []common.Address
	minProfit  *big.Int
	logger     *zap.Logger
	simulator  *simulator.Simulator
	mu         sync.Mutex
	minProfitThreshold *big.Int
}

// NewDetector creates a new arbitrage detector
func NewDetector(client *ethclient.Client, exchanges []dex.Exchange, tokens []common.Address, minProfit *big.Int, logger *zap.Logger) *Detector {
	return &Detector{
		client:     client,
		exchanges:  exchanges,
		tokens:     tokens,
		minProfit:  minProfit,
		minProfitThreshold: big.NewInt(0),
		logger:     logger,
		simulator:  simulator.NewSimulator(client),
	}
}

// FindArbitrage finds arbitrage opportunities between exchanges
func (d *Detector) FindArbitrage(ctx context.Context, baseToken common.Address) ([]*Route, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var routes []*Route

	// For each token pair
	for _, token0 := range d.tokens {
		for _, token1 := range d.tokens {
			if token0 == token1 {
				continue
			}

			// Check prices across exchanges
			for i, exchange0 := range d.exchanges {
				for j, exchange1 := range d.exchanges {
					if i == j {
						continue
					}

					// Get prices from both exchanges
					price0, err := exchange0.GetPrice(ctx, token0, token1)
					if err != nil {
						d.logger.Error("Failed to get price", 
							zap.String("exchange", exchange0.GetName()),
							zap.Error(err))
						continue
					}

					price1, err := exchange1.GetPrice(ctx, token1, token0)
					if err != nil {
						d.logger.Error("Failed to get price",
							zap.String("exchange", exchange1.GetName()),
							zap.Error(err))
						continue
					}

					// Calculate potential profit
					profit := new(big.Int).Sub(price1, price0)
					if profit.Cmp(d.minProfit) > 0 {
						// Create route
						route := &Route{
							Path:         []common.Address{token0, token1},
							Exchanges:    []dex.Exchange{exchange0, exchange1},
							InputAmount:  big.NewInt(1e18), // Start with 1 token
							OutputAmount: new(big.Int).Add(big.NewInt(1e18), profit),
							Profit:      profit,
						}

						// Simulate the trade
						simResult, err := d.simulator.SimulateArbitrage(ctx, route)
						if err != nil {
							d.logger.Error("Failed to simulate arbitrage",
								zap.Error(err))
							continue
						}

						route.Simulated = true
						route.SimulationGas = simResult.GasUsed
						route.GasCost = new(big.Int).Mul(
							big.NewInt(int64(simResult.GasUsed)),
							simResult.GasPrice,
						)

						// Only add route if still profitable after gas
						if new(big.Int).Sub(route.Profit, route.GasCost).Cmp(d.minProfit) > 0 {
							routes = append(routes, route)
						}
					}
				}
			}
		}
	}

	return routes, nil
}

// DetectOpportunities detects arbitrage opportunities
func (d *Detector) DetectOpportunities(
	exchange1Rates map[common.Address]map[common.Address]*big.Int,
	exchange2Rates map[common.Address]map[common.Address]*big.Int,
) []*types.ArbitrageOpportunity {
	var opportunities []*types.ArbitrageOpportunity

	// For each token pair
	for token0 := range exchange1Rates {
		for token1 := range exchange1Rates[token0] {
			if token0 == token1 {
				continue
			}

			// Check prices across exchanges
			price0 := exchange1Rates[token0][token1]
			price1 := exchange2Rates[token1][token0]

			// Calculate potential profit
			profit := new(big.Int).Sub(price1, price0)
			if profit.Cmp(d.minProfitThreshold) > 0 {
				// Create opportunity
				opportunity := &types.ArbitrageOpportunity{
					Token0: token0,
					Token1: token1,
					Price0: price0,
					Price1: price1,
					Profit: profit,
				}

				opportunities = append(opportunities, opportunity)
			}
		}
	}

	return opportunities
}

// Stop stops the detector
func (d *Detector) Stop() {
	// Cleanup resources
}
