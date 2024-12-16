package mempool

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// CPUSet represents a set of CPU cores
type CPUSet struct {
	mask unix.CPUSet
	mu   sync.Mutex
}

// NewCPUSet creates a new CPU set
func NewCPUSet() *CPUSet {
	var set CPUSet
	unix.SchedGetaffinity(0, &set.mask)
	return &set
}

// Pin pins the current goroutine to the specified CPU core
func (cs *CPUSet) Pin(cpu int) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	var mask unix.CPUSet
	mask.Zero()
	mask.Set(cpu)

	// Set thread affinity
	if err := unix.SchedSetaffinity(0, &mask); err != nil {
		return fmt.Errorf("failed to set CPU affinity: %w", err)
	}

	return nil
}

// AffinityWorkerPool is a worker pool with CPU affinity
type AffinityWorkerPool struct {
	workers    int
	queue      chan func()
	wg         sync.WaitGroup
	cpuSet     *CPUSet
	startIndex atomic.Int32
	metrics    struct {
		taskLatency   float64
		queueLength   int64
		activeWorkers int32
	}
}

// NewAffinityWorkerPool creates a new worker pool with CPU affinity
func NewAffinityWorkerPool(config *WorkerConfig) *AffinityWorkerPool {
	return &AffinityWorkerPool{
		workers: config.Workers,
		queue:   make(chan func(), config.QueueSize),
		cpuSet:  NewCPUSet(),
	}
}

// Start starts the worker pool
func (p *AffinityWorkerPool) Start() error {
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
func (p *AffinityWorkerPool) Stop() error {
	close(p.queue)
	p.wg.Wait()
	return nil
}

// Submit submits a task to the worker pool
func (p *AffinityWorkerPool) Submit(task func()) {
	atomic.AddInt64(&p.metrics.queueLength, 1)
	p.queue <- task
}

// worker is a CPU-pinned worker goroutine
func (p *AffinityWorkerPool) worker(id int) {
	defer p.wg.Done()

	// Pin this worker to a specific CPU core
	// We use round-robin assignment starting from the last used core
	startCore := int(p.startIndex.Add(1)) % runtime.NumCPU()
	targetCore := (startCore + id) % runtime.NumCPU()

	if err := p.cpuSet.Pin(targetCore); err != nil {
		// Log error but continue - the worker will still function without affinity
		fmt.Printf("Failed to pin worker %d to CPU %d: %v\n", id, targetCore, err)
	}

	// Pre-allocate timer for task latency measurement
	timer := new(timer)

	for task := range p.queue {
		atomic.AddInt32(&p.metrics.activeWorkers, 1)
		timer.start()

		// Execute the task
		task()

		// Update metrics
		latency := timer.elapsed()
		atomic.StoreInt64(&p.metrics.queueLength, int64(len(p.queue)))
		atomic.AddInt32(&p.metrics.activeWorkers, -1)

		// Update average latency using exponential moving average
		oldLatency := atomic.LoadFloat64((*float64)(&p.metrics.taskLatency))
		newLatency := oldLatency*0.9 + float64(latency)*0.1
		atomic.StoreFloat64((*float64)(&p.metrics.taskLatency), newLatency)
	}
}

// timer is a simple timer for measuring task latency
type timer struct {
	start int64
}

func (t *timer) start() {
	t.start = unix.TimevalToNsec(unix.NsecToTimeval(unix.NowNsec()))
}

func (t *timer) elapsed() int64 {
	now := unix.TimevalToNsec(unix.NsecToTimeval(unix.NowNsec()))
	return now - t.start
}

// GetMetrics returns the current pool metrics
func (p *AffinityWorkerPool) GetMetrics() (queueLength int64, activeWorkers int32, avgLatencyNs float64) {
	return atomic.LoadInt64(&p.metrics.queueLength),
		atomic.LoadInt32(&p.metrics.activeWorkers),
		atomic.LoadFloat64((*float64)(&p.metrics.taskLatency))
}
