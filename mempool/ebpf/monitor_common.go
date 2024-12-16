package ebpf

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// SyscallMonitor interface defines the common functionality across platforms
type SyscallMonitor interface {
	Start(ctx context.Context) error
	Stop() error
	GetStats() (slowSyscalls uint64, avgLatencyNs uint64, maxLatencyNs uint64)
}

// BaseMonitor contains the common fields used by both Linux and Darwin implementations
type BaseMonitor struct {
	logger *zap.Logger
	
	// Statistics
	slowSyscalls   atomic.Uint64
	totalLatencyNs atomic.Uint64
	maxLatencyNs   atomic.Uint64

	// Cleanup
	wg   sync.WaitGroup
	done chan struct{}
	mu   sync.RWMutex
}

// collectStats periodically collects and logs statistics
func (m *BaseMonitor) collectStats(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.done:
			return
		case <-ticker.C:
			m.mu.RLock()
			count := m.slowSyscalls.Load()
			if count == 0 {
				m.mu.RUnlock()
				continue
			}

			total := m.totalLatencyNs.Load()
			max := m.maxLatencyNs.Load()
			avg := total / count

			m.mu.RUnlock()

			m.logger.Info("Syscall statistics",
				zap.Uint64("slow_syscalls", count),
				zap.Duration("avg_latency", time.Duration(avg)),
				zap.Duration("max_latency", time.Duration(max)),
			)
		}
	}
}

// GetStats returns the current monitoring statistics
func (m *BaseMonitor) GetStats() (slowSyscalls uint64, avgLatencyNs uint64, maxLatencyNs uint64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := m.slowSyscalls.Load()
	if count == 0 {
		return 0, 0, 0
	}

	total := m.totalLatencyNs.Load()
	return count, total / count, m.maxLatencyNs.Load()
}
