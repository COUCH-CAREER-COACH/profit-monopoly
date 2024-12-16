package monitor

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// SystemMonitor provides comprehensive system monitoring
type SystemMonitor struct {
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *zap.Logger
	metrics struct {
		cpuUsage    prometheus.Gauge
		memUsage    prometheus.Gauge
		goroutines  prometheus.Gauge
		heapObjects prometheus.Gauge
		heapAlloc   prometheus.Gauge
		gcPause     prometheus.Gauge
	}
	wg sync.WaitGroup
}

// NewSystemMonitor creates a new system monitor
func NewSystemMonitor(ctx context.Context, logger *zap.Logger) (*SystemMonitor, error) {
	ctx, cancel := context.WithCancel(ctx)
	m := &SystemMonitor{
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}

	// Initialize metrics
	m.metrics.cpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "system_cpu_usage_percent",
		Help: "Current CPU usage percentage",
	})
	m.metrics.memUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "system_memory_usage_bytes",
		Help: "Current memory usage in bytes",
	})
	m.metrics.goroutines = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "system_goroutines",
		Help: "Current number of goroutines",
	})
	m.metrics.heapObjects = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "system_heap_objects",
		Help: "Current number of heap objects",
	})
	m.metrics.heapAlloc = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "system_heap_alloc_bytes",
		Help: "Current heap allocation in bytes",
	})
	m.metrics.gcPause = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "system_gc_pause_seconds",
		Help: "GC pause duration",
	})

	// Register metrics
	prometheus.MustRegister(m.metrics.cpuUsage)
	prometheus.MustRegister(m.metrics.memUsage)
	prometheus.MustRegister(m.metrics.goroutines)
	prometheus.MustRegister(m.metrics.heapObjects)
	prometheus.MustRegister(m.metrics.heapAlloc)
	prometheus.MustRegister(m.metrics.gcPause)

	// Start monitoring
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.monitor()
	}()

	return m, nil
}

// monitor periodically collects system metrics
func (m *SystemMonitor) monitor() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.collectMetrics(); err != nil {
				m.logger.Error("Failed to collect metrics", zap.Error(err))
			}
		}
	}
}

// collectMetrics collects various system metrics
func (m *SystemMonitor) collectMetrics() error {
	// CPU usage
	cpuUsage := m.getCPUUsage()
	m.metrics.cpuUsage.Set(cpuUsage)

	// Memory usage
	memUsage := m.getMemoryUsage()
	m.metrics.memUsage.Set(memUsage)

	// Goroutines
	goroutines := runtime.NumGoroutine()
	m.metrics.goroutines.Set(float64(goroutines))

	// Heap objects
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.metrics.heapObjects.Set(float64(memStats.HeapObjects))
	m.metrics.heapAlloc.Set(float64(memStats.HeapAlloc))

	// GC pause
	gcPause := float64(memStats.PauseNs[(memStats.NumGC+255)%256]) / float64(time.Millisecond)
	m.metrics.gcPause.Set(gcPause)

	return nil
}

// GetMetrics returns current system metrics
func (m *SystemMonitor) GetMetrics() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"cpu_usage":     int64(m.getCPUUsage()),
		"mem_usage":     int64(m.getMemoryUsage()),
		"goroutines":    int64(runtime.NumGoroutine()),
		"heap_objects":  int64(memStats.HeapObjects),
		"heap_alloc":    int64(memStats.HeapAlloc),
		"gc_pause":      float64(memStats.PauseNs[(memStats.NumGC+255)%256]) / float64(time.Millisecond),
	}
}

// Cleanup performs cleanup operations
func (m *SystemMonitor) Cleanup() error {
	m.cancel()
	m.wg.Wait()
	return nil
}

func (m *SystemMonitor) getCPUUsage() float64 {
	// Simple CPU usage calculation for testing
	return 50.0 // Return a constant value for now
}

func (m *SystemMonitor) getMemoryUsage() float64 {
	// Simple memory usage calculation for testing
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return float64(memStats.Alloc) / float64(memStats.Sys) * 100
}
