//go:build linux
// +build linux

package network

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestEBPFOptimizer(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges for eBPF")
	}

	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	t.Run("initialization", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.System.EBPFEnabled = true
		cfg.System.EBPFProgramPath = "testdata/ebpf_program.o"

		opt, err := NewEBPFOptimizer(ctx, logger, cfg)
		require.NoError(t, err)
		defer opt.Cleanup()

		// Verify initialization
		assert.NotEmpty(t, opt.programs)
		assert.NotEmpty(t, opt.links)
	})

	t.Run("program_loading", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.System.EBPFEnabled = true
		cfg.System.EBPFProgramPath = "testdata/ebpf_program.o"

		opt, err := NewEBPFOptimizer(ctx, logger, cfg)
		require.NoError(t, err)
		defer opt.Cleanup()

		// Verify program loading
		assert.Contains(t, opt.programs, "syscall_tracer")
		assert.Contains(t, opt.programs, "network_optimizer")
	})

	t.Run("metrics", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.System.EBPFEnabled = true
		cfg.System.EBPFProgramPath = "testdata/ebpf_program.o"

		opt, err := NewEBPFOptimizer(ctx, logger, cfg)
		require.NoError(t, err)
		defer opt.Cleanup()

		// Wait for metrics collection
		time.Sleep(2 * time.Second)

		// Verify metrics
		assert.NotZero(t, opt.metrics.syscallLatency.Count())
		assert.NotZero(t, opt.metrics.eventCount.Count())
	})

	t.Run("cleanup", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.System.EBPFEnabled = true
		cfg.System.EBPFProgramPath = "testdata/ebpf_program.o"

		opt, err := NewEBPFOptimizer(ctx, logger, cfg)
		require.NoError(t, err)

		// Verify cleanup
		err = opt.Cleanup()
		assert.NoError(t, err)
		assert.Empty(t, opt.programs)
		assert.Empty(t, opt.links)
	})
}

func TestEBPFOptimizerDisabled(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	t.Run("initialization_disabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.System.EBPFEnabled = false

		opt, err := NewEBPFOptimizer(ctx, logger, cfg)
		require.NoError(t, err)
		defer opt.Cleanup()

		// Verify no eBPF initialization
		assert.Empty(t, opt.programs)
		assert.Empty(t, opt.links)
	})
}

func BenchmarkEBPFOptimizer(b *testing.B) {
	if os.Getuid() != 0 {
		b.Skip("Benchmark requires root privileges for eBPF")
	}

	logger := zap.NewNop()
	ctx := context.Background()
	cfg := DefaultConfig()
	cfg.System.EBPFEnabled = true
	cfg.System.EBPFProgramPath = "testdata/ebpf_program.o"

	opt, err := NewEBPFOptimizer(ctx, logger, cfg)
	require.NoError(b, err)
	defer opt.Cleanup()

	b.Run("event_processing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate event processing
			opt.metrics.eventCount.Inc(1)
			opt.metrics.syscallLatency.Update(time.Microsecond)
		}
	})
}
