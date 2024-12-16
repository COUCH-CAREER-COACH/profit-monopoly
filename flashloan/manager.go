package flashloan

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.uber.org/zap"
)

// FlashLoanManager coordinates flash loan operations across providers
type FlashLoanManager struct {
	mu        sync.RWMutex
	metrics   struct {
		providerSelections prometheus.CounterVec
		executionLatency  prometheus.Histogram
		successRate       prometheus.Gauge
		totalVolume      prometheus.Counter
		activeLoans      prometheus.Gauge
		successCount     prometheus.Counter
		totalCount       prometheus.Counter
		errors           prometheus.CounterVec
	}
	providers []Provider
	logger    *zap.Logger
}

// NewFlashLoanManager creates a new flash loan manager
func NewFlashLoanManager(logger *zap.Logger) *FlashLoanManager {
	manager := &FlashLoanManager{
		logger: logger,
	}

	// Initialize metrics
	manager.metrics.providerSelections = *prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashloan_provider_selections_total",
		Help: "Number of times each provider was selected",
	}, []string{"provider"})

	manager.metrics.executionLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "flashloan_execution_latency_seconds",
		Help:    "Latency of flash loan execution",
		Buckets: prometheus.DefBuckets,
	})

	manager.metrics.successRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flashloan_success_rate",
		Help: "Success rate of flash loan executions",
	})

	manager.metrics.totalVolume = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flashloan_total_volume",
		Help: "Total volume of flash loans executed",
	})

	manager.metrics.activeLoans = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flashloan_active_loans",
		Help: "Number of currently active flash loans",
	})

	manager.metrics.successCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flashloan_success_count",
		Help: "Number of successful flash loan executions",
	})

	manager.metrics.totalCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flashloan_total_count",
		Help: "Total number of flash loan executions",
	})

	manager.metrics.errors = *prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flashloan_errors_total",
		Help: "Number of flash loan errors by type",
	}, []string{"error_type"})

	return manager
}

// ExecuteFlashLoan executes a flash loan with optimal provider selection
func (m *FlashLoanManager) ExecuteFlashLoan(ctx context.Context, params FlashLoanParams) (*types.Transaction, error) {
	start := time.Now()
	defer func() {
		m.metrics.executionLatency.Observe(time.Since(start).Seconds())
	}()

	// Increment active loans counter
	m.metrics.activeLoans.Inc()
	defer m.metrics.activeLoans.Dec()

	// Find best provider
	provider, err := m.selectOptimalProvider(ctx, params)
	if err != nil {
		m.metrics.errors.WithLabelValues("provider_selection").Inc()
		return nil, fmt.Errorf("failed to select provider: %w", err)
	}

	// Calculate flash loan fee
	loanAmount := new(big.Int)
	if _, ok := loanAmount.SetString(params.Amount.String(), 10); !ok {
		return nil, fmt.Errorf("invalid loan amount")
	}

	// Get current fee from provider
	fee, err := provider.GetFlashLoanFee(ctx, params.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to get flash loan fee: %w", err)
	}

	// Calculate total repayment amount including fee
	totalRepayment := new(big.Int).Add(loanAmount, fee)

	// Execute flash loan
	tx, err := provider.ExecuteFlashLoan(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute flash loan: %w", err)
	}

	// Update metrics
	m.metrics.totalVolume.Add(float64(totalRepayment.Uint64()))
	m.metrics.successCount.Inc()
	m.updateSuccessRate()

	return tx, nil
}

// AddProvider adds a new flash loan provider
func (m *FlashLoanManager) AddProvider(provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers = append(m.providers, provider)
}

// selectOptimalProvider selects the best provider based on fees and liquidity
func (m *FlashLoanManager) selectOptimalProvider(ctx context.Context, params FlashLoanParams) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	var (
		bestProvider Provider
		bestFee     *big.Int
	)

	for _, provider := range m.providers {
		// Get provider fee
		fee, err := provider.GetFlashLoanFee(ctx, params.Token)
		if err != nil {
			m.logger.Warn("Failed to get provider fee", zap.Error(err))
			continue
		}

		// Check if this is the best fee so far
		if bestFee == nil || fee.Cmp(bestFee) < 0 {
			bestProvider = provider
			bestFee = fee
		}
	}

	if bestProvider == nil {
		return nil, fmt.Errorf("no suitable provider found")
	}

	m.metrics.providerSelections.WithLabelValues(bestProvider.String()).Inc()
	return bestProvider, nil
}

// updateSuccessRate updates the success rate metric
func (m *FlashLoanManager) updateSuccessRate() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get current success rate from collectors
	var successCount, totalCount float64

	// Update success rate using direct counter values
	m.metrics.successCount.Add(1)
	m.metrics.totalCount.Add(1)
	
	// Get metric values using prometheus.Collector interface
	ch := make(chan prometheus.Metric, 1)
	m.metrics.successCount.(prometheus.Collector).Collect(ch)
	if metric := <-ch; metric != nil {
		dto := &dto.Metric{}
		if err := metric.Write(dto); err == nil && dto.Counter != nil {
			successCount = *dto.Counter.Value
		}
	}

	m.metrics.totalCount.(prometheus.Collector).Collect(ch)
	if metric := <-ch; metric != nil {
		dto := &dto.Metric{}
		if err := metric.Write(dto); err == nil && dto.Counter != nil {
			totalCount = *dto.Counter.Value
		}
	}

	if totalCount > 0 {
		m.metrics.successRate.Set(successCount / totalCount)
	}
}
