//go:build darwin
// +build darwin

package ebpf

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DarwinMonitor implements SyscallMonitor for Darwin systems
type DarwinMonitor struct {
	logger *zap.Logger
	wg    sync.WaitGroup
	done  chan struct{}

	// Statistics
	slowSyscalls   uint64
	totalLatencyNs uint64
	maxLatencyNs   uint64
}

// NewSyscallMonitor creates a new syscall monitor for Darwin
func NewSyscallMonitor(logger *zap.Logger) (*DarwinMonitor, error) {
	m := &DarwinMonitor{
		logger: logger,
		done:   make(chan struct{}),
	}
	return m, nil
}

// Start starts monitoring (no-op on Darwin)
func (m *DarwinMonitor) Start(ctx context.Context) error {
	// Start statistics collection
	m.wg.Add(1)
	go m.collectStats(ctx)
	return nil
}

// Stop stops monitoring
func (m *DarwinMonitor) Stop() error {
	close(m.done)
	m.wg.Wait()
	return nil
}

// collectStats periodically collects and logs statistics
func (m *DarwinMonitor) collectStats(ctx context.Context) {
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
			count := m.slowSyscalls
			if count == 0 {
				continue
			}

			total := m.totalLatencyNs
			max := m.maxLatencyNs
			avg := total / count

			m.logger.Info("Syscall statistics",
				zap.Uint64("slow_syscalls", count),
				zap.Duration("avg_latency", time.Duration(avg)),
				zap.Duration("max_latency", time.Duration(max)),
			)
		}
	}
}

// GetStats returns the current monitoring statistics
func (m *DarwinMonitor) GetStats() (slowSyscalls uint64, avgLatencyNs uint64, maxLatencyNs uint64) {
	count := m.slowSyscalls
	if count == 0 {
		return 0, 0, 0
	}

	total := m.totalLatencyNs
	return count, total / count, m.maxLatencyNs
}
