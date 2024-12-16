//go:build linux
// +build linux

package network

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// EBPFOptimizer provides system call and network optimizations using eBPF
type EBPFOptimizer struct {
	ctx     context.Context
	cancel  context.CancelFunc
	logger  *zap.Logger
	config  *Config
	mu      sync.RWMutex
	metrics struct {
		syscallLatency prometheus.Histogram
		syscallErrors  prometheus.Counter
		programLoads   prometheus.Counter
		eventCount     prometheus.Counter
	}
	programs map[string]*ebpf.Program
	links    []link.Link
}

// NewEBPFOptimizer creates a new eBPF optimizer
func NewEBPFOptimizer(ctx context.Context, logger *zap.Logger, config *Config) (*EBPFOptimizer, error) {
	ctx, cancel := context.WithCancel(ctx)

	opt := &EBPFOptimizer{
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
		config:   config,
		programs: make(map[string]*ebpf.Program),
	}

	// Initialize metrics
	opt.metrics.syscallLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ebpf_syscall_latency_seconds",
		Help:    "Histogram of eBPF syscall latencies in seconds",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10), // Start at 0.1ms, double 10 times
	})
	opt.metrics.syscallErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ebpf_syscall_errors_total",
		Help: "Total number of eBPF syscall errors",
	})
	opt.metrics.programLoads = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ebpf_program_loads_total",
		Help: "Total number of eBPF program loads",
	})
	opt.metrics.eventCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ebpf_events_total",
		Help: "Total number of eBPF events",
	})

	if err := opt.init(); err != nil {
		cancel()
		return nil, err
	}

	return opt, nil
}

// init initializes the eBPF optimizer
func (o *EBPFOptimizer) init() error {
	if !o.config.System.EBPFEnabled {
		return nil
	}

	// Load eBPF program
	spec, err := ebpf.LoadCollectionSpec(o.config.System.EBPFProgramPath)
	if err != nil {
		return fmt.Errorf("failed to load eBPF program: %w", err)
	}

	// Create programs
	var objs struct {
		SyscallTracer    *ebpf.Program `ebpf:"syscall_tracer"`
		NetworkOptimizer *ebpf.Program `ebpf:"network_optimizer"`
	}

	if err := spec.LoadAndAssign(&objs, nil); err != nil {
		return fmt.Errorf("failed to load eBPF objects: %w", err)
	}

	o.programs["syscall_tracer"] = objs.SyscallTracer
	o.programs["network_optimizer"] = objs.NetworkOptimizer

	// Attach programs
	if err := o.attachPrograms(); err != nil {
		return fmt.Errorf("failed to attach eBPF programs: %w", err)
	}

	// Start event monitoring
	go o.monitorEvents()

	o.logger.Info("eBPF optimizer initialized successfully",
		zap.Int("programs", len(o.programs)))

	return nil
}

// attachPrograms attaches eBPF programs to appropriate hooks
func (o *EBPFOptimizer) attachPrograms() error {
	// Attach syscall tracer
	syscallLink, err := link.AttachTracing(link.TracingOptions{
		Program: o.programs["syscall_tracer"],
	})
	if err != nil {
		return fmt.Errorf("failed to attach syscall tracer: %w", err)
	}
	o.links = append(o.links, syscallLink)

	// Attach network optimizer
	netLink, err := link.AttachXDP(link.XDPOptions{
		Program: o.programs["network_optimizer"],
	})
	if err != nil {
		return fmt.Errorf("failed to attach network optimizer: %w", err)
	}
	o.links = append(o.links, netLink)

	o.metrics.programLoads.Inc()
	return nil
}

// monitorEvents monitors eBPF events
func (o *EBPFOptimizer) monitorEvents() {
	// Open ring buffer reader
	rd, err := ringbuf.NewReader(o.programs["syscall_tracer"])
	if err != nil {
		o.logger.Error("Failed to create ring buffer reader", zap.Error(err))
		return
	}
	defer rd.Close()

	var event struct {
		Pid      uint32
		Syscall  uint32
		Latency  uint64
		ErrorNum int32
	}

	for {
		select {
		case <-o.ctx.Done():
			return
		default:
			record, err := rd.Read()
			if err != nil {
				if err == ringbuf.ErrClosed {
					return
				}
				o.logger.Error("Error reading from ring buffer", zap.Error(err))
				continue
			}

			// Parse event
			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
				o.logger.Error("Error parsing event", zap.Error(err))
				continue
			}

			// Update metrics
			o.metrics.eventCount.Inc()
			o.metrics.syscallLatency.Observe(float64(event.Latency) / 1e9)
			if event.ErrorNum != 0 {
				o.metrics.syscallErrors.Inc()
			}
		}
	}
}

// Cleanup cleans up eBPF resources
func (o *EBPFOptimizer) Cleanup() error {
	o.cancel()

	// Detach programs
	for _, link := range o.links {
		if err := link.Close(); err != nil {
			o.logger.Error("Failed to close eBPF link", zap.Error(err))
		}
	}

	// Close programs
	for name, prog := range o.programs {
		if err := prog.Close(); err != nil {
			o.logger.Error("Failed to close eBPF program",
				zap.String("name", name),
				zap.Error(err))
		}
	}

	return nil
}
