package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSystemMonitor(t *testing.T) {
	// Create test logger
	logger, err := zap.NewDevelopment()
	assert.NoError(t, err)
	defer logger.Sync()

	// Create monitor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mon, err := NewSystemMonitor(ctx, logger)
	assert.NoError(t, err)
	assert.NotNil(t, mon)

	// Let it collect some metrics
	time.Sleep(2 * time.Second)

	// Get metrics
	metrics := mon.GetMetrics()
	assert.NotNil(t, metrics)

	// Check required metrics exist
	assert.Contains(t, metrics, "cpu_usage")
	assert.Contains(t, metrics, "mem_usage")
	assert.Contains(t, metrics, "goroutines")
	assert.Contains(t, metrics, "heap_objects")
	assert.Contains(t, metrics, "heap_alloc")
	assert.Contains(t, metrics, "gc_pause")

	// Check metric values are reasonable
	cpuUsage, ok := metrics["cpu_usage"].(int64)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, cpuUsage, int64(0))

	memUsage, ok := metrics["mem_usage"].(int64)
	assert.True(t, ok)
	assert.Greater(t, memUsage, int64(0))

	goroutines, ok := metrics["goroutines"].(int64)
	assert.True(t, ok)
	assert.Greater(t, goroutines, int64(0))

	heapObjects, ok := metrics["heap_objects"].(int64)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, heapObjects, int64(0))

	heapAlloc, ok := metrics["heap_alloc"].(int64)
	assert.True(t, ok)
	assert.Greater(t, heapAlloc, int64(0))

	gcPause, ok := metrics["gc_pause"].(float64)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, gcPause, float64(0))

	// Test cleanup
	err = mon.Cleanup()
	assert.NoError(t, err)

	// Verify metrics are still accessible after cleanup
	metrics = mon.GetMetrics()
	assert.NotNil(t, metrics)
}

func BenchmarkSystemMonitor(b *testing.B) {
	logger := zap.NewNop()
	ctx := context.Background()

	mon, err := NewSystemMonitor(ctx, logger)
	require.NoError(b, err)
	defer mon.Cleanup()

	b.Run("metrics_collection", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := mon.collectMetrics()
			require.NoError(b, err)
		}
	})

	b.Run("get_metrics", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = mon.GetMetrics()
		}
	})
}
