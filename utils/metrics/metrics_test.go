package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestMetricsInitialization(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := &MetricsConfig{
		ReportInterval: time.Second,
		LogMetrics:     true,
	}

	Initialize(cfg, logger)
	assert.NotNil(t, registry)
}

func TestNetworkMetrics(t *testing.T) {
	metrics := NewNetworkMetrics("test_network")
	assert.NotNil(t, metrics)

	// Test counter operations
	metrics.Errors.Inc()
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.Errors))

	metrics.BytesSent.Add(100)
	assert.Equal(t, float64(100), testutil.ToFloat64(metrics.BytesSent))

	// Test histogram operations
	metrics.SendLatency.Observe(0.1)
	// For histograms, we can only verify that they were created and can accept observations
	assert.NotNil(t, metrics.SendLatency)
}

func TestMempoolMetrics(t *testing.T) {
	metrics := NewMempoolMetrics("test_mempool")
	assert.NotNil(t, metrics)

	// Test counter operations
	metrics.TxCount.Inc()
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.TxCount))

	// Test histogram operations
	metrics.TxSize.Observe(1024)
	assert.NotNil(t, metrics.TxSize)

	metrics.ProcessTime.Observe(0.1)
	assert.NotNil(t, metrics.ProcessTime)
}

func TestSystemMetrics(t *testing.T) {
	metrics := NewSystemMetrics("test_system")
	assert.NotNil(t, metrics)

	// Test gauge operations
	metrics.CPUUsage.Set(50)
	assert.Equal(t, float64(50), testutil.ToFloat64(metrics.CPUUsage))

	// Test counter operations
	metrics.DiskIO.Add(1024)
	assert.Equal(t, float64(1024), testutil.ToFloat64(metrics.DiskIO))
}

func TestStrategyMetrics(t *testing.T) {
	metrics := NewStrategyMetrics("test_strategy")
	assert.NotNil(t, metrics)

	// Test counter operations
	metrics.Attempts.Inc()
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.Attempts))

	metrics.Successes.Inc()
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.Successes))

	// Test histogram operations
	metrics.GasUsed.Observe(21000)
	assert.NotNil(t, metrics.GasUsed)

	metrics.ExecutionTime.Observe(0.1)
	assert.NotNil(t, metrics.ExecutionTime)

	// Test counter operations for specific strategies
	metrics.Frontrunning.Inc()
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.Frontrunning))

	metrics.Backrunning.Inc()
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.Backrunning))

	metrics.Sandwiching.Inc()
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.Sandwiching))
}
