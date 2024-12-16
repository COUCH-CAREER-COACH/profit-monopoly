package mempool

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// CPUSet represents a set of CPU cores
type CPUSet struct {
	mu sync.Mutex
}

// NewCPUSet creates a new CPU set
func NewCPUSet() *CPUSet {
	return &CPUSet{}
}

// Pin pins the current goroutine to the specified CPU core
// Note: CPU pinning is not supported on Darwin/macOS
func (cs *CPUSet) Pin(cpu int) error {
	// CPU pinning not supported on Darwin
	return nil
}

// AffinityWorkerPool2 is a worker pool with CPU affinity
type AffinityWorkerPool2 struct {
	workers    int
	queue      chan func()
	wg         sync.WaitGroup
	cpuSet     *CPUSet
	startIndex atomic.Int32
	metrics    struct {
		taskLatency   atomic.Int64
		queueLength   atomic.Int64
		activeWorkers atomic.Int32
	}
}

// NewAffinityWorkerPool2 creates a new worker pool with CPU affinity
func NewAffinityWorkerPool2(config *WorkerConfig) *AffinityWorkerPool2 {
	return &AffinityWorkerPool2{
		workers: config.NumWorkers,
		queue:   make(chan func(), config.QueueSize),
		cpuSet:  NewCPUSet(),
	}
}

// Start starts the worker pool
func (p *AffinityWorkerPool2) Start() error {
	// Get available CPU cores
	numCPU := runtime.NumCPU()
	if p.workers > numCPU {
		return fmt.Errorf("worker count (%d) exceeds available CPUs (%d)", p.workers, numCPU)
	}

	// Start workers with CPU affinity
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	return nil
}

// Stop stops the worker pool
func (p *AffinityWorkerPool2) Stop() error {
	close(p.queue)
	p.wg.Wait()
	return nil
}

// Submit submits a task to the worker pool
func (p *AffinityWorkerPool2) Submit(task func()) {
	p.metrics.queueLength.Add(1)
	p.queue <- task
}

// worker is a CPU-pinned worker goroutine
func (p *AffinityWorkerPool2) worker(id int) {
	defer p.wg.Done()

	// Pin this worker to a specific CPU core
	// We use round-robin assignment starting from the last used core
	startCore := int(p.startIndex.Add(1)) % runtime.NumCPU()
	targetCore := (startCore + id) % runtime.NumCPU()

	if err := p.cpuSet.Pin(targetCore); err != nil {
		// Log error but continue - the worker will still function without affinity
		fmt.Printf("Failed to pin worker %d to CPU %d: %v\n", id, targetCore, err)
	}

	for task := range p.queue {
		p.metrics.activeWorkers.Add(1)
		start := time.Now().UnixNano()

		// Execute the task
		task()

		// Update metrics
		latency := time.Now().UnixNano() - start
		p.metrics.queueLength.Store(int64(len(p.queue)))
		p.metrics.taskLatency.Store(latency)
		p.metrics.activeWorkers.Add(-1)
	}
}

// GetMetrics returns the current pool metrics
func (p *AffinityWorkerPool2) GetMetrics() (queueLength int64, activeWorkers int32, avgLatencyNs int64) {
	return p.metrics.queueLength.Load(),
		p.metrics.activeWorkers.Load(),
		p.metrics.taskLatency.Load()
}
