//go:build linux
// +build linux

package ebpf

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
	"go.uber.org/zap"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpfel -type event -type latency mevbot mevbot.bpf.c -- -I/usr/include/bpf

// LinuxMonitor implements SyscallMonitor for Linux systems using eBPF
type LinuxMonitor struct {
	logger *zap.Logger
	wg    sync.WaitGroup
	done  chan struct{}

	// Statistics
	slowSyscalls   uint64
	totalLatencyNs uint64
	maxLatencyNs   uint64

	// eBPF resources
	objs  mevbotObjects
	rb    *ringbuf.Reader
	links []link.Link
}

// NewSyscallMonitor creates a new eBPF-based syscall monitor for Linux
func NewSyscallMonitor(logger *zap.Logger) (SyscallMonitor, error) {
	// Allow the current process to lock memory for eBPF resources
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("failed to remove memlock limit: %w", err)
	}

	// Load pre-compiled programs
	objs := mevbotObjects{}
	if err := loadMevbotObjects(&objs, nil); err != nil {
		return nil, fmt.Errorf("failed to load objects: %w", err)
	}

	// Create ring buffer reader
	rb, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		objs.Close()
		return nil, fmt.Errorf("failed to create ring buffer reader: %w", err)
	}

	m := &LinuxMonitor{
		logger: logger,
		done:   make(chan struct{}),
		objs:   objs,
		rb:     rb,
	}

	return m, nil
}

// Start attaches the eBPF programs and starts monitoring
func (m *LinuxMonitor) Start(ctx context.Context) error {
	// Attach tracepoints
	enterLink, err := link.Tracepoint("raw_syscalls", "sys_enter", m.objs.TraceEnter, nil)
	if err != nil {
		return fmt.Errorf("failed to attach sys_enter: %w", err)
	}
	m.links = append(m.links, enterLink)

	exitLink, err := link.Tracepoint("raw_syscalls", "sys_exit", m.objs.TraceExit, nil)
	if err != nil {
		enterLink.Close()
		return fmt.Errorf("failed to attach sys_exit: %w", err)
	}
	m.links = append(m.links, exitLink)

	socketLink, err := link.Tracepoint("syscalls", "sys_enter_socket", m.objs.TraceSocket, nil)
	if err != nil {
		for _, link := range m.links {
			link.Close()
		}
		return fmt.Errorf("failed to attach socket trace: %w", err)
	}
	m.links = append(m.links, socketLink)

	// Start event processing
	m.wg.Add(1)
	go m.processEvents(ctx)

	// Start statistics collection
	m.wg.Add(1)
	go m.collectStats(ctx)

	return nil
}

// Stop detaches the eBPF programs and stops monitoring
func (m *LinuxMonitor) Stop() error {
	close(m.done)
	
	// Close all links
	for _, link := range m.links {
		link.Close()
	}

	// Close ring buffer reader
	m.rb.Close()

	// Wait for goroutines to finish
	m.wg.Wait()

	// Close eBPF objects
	m.objs.Close()

	return nil
}

// processEvents reads and processes events from the ring buffer
func (m *LinuxMonitor) processEvents(ctx context.Context) {
	defer m.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.done:
			return
		default:
			record, err := m.rb.Read()
			if err != nil {
				if errors.Is(err, ringbuf.ErrClosed) {
					return
				}
				m.logger.Error("Error reading from ring buffer", zap.Error(err))
				continue
			}

			// Parse event
			var event mevbotEvent
			if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
				m.logger.Error("Error parsing event", zap.Error(err))
				continue
			}

			// Update statistics
			atomic.AddUint64(&m.slowSyscalls, 1)
			atomic.AddUint64(&m.totalLatencyNs, event.DurationNs)
			
			// Update max latency if needed
			for {
				current := atomic.LoadUint64(&m.maxLatencyNs)
				if event.DurationNs <= current {
					break
				}
				if atomic.CompareAndSwapUint64(&m.maxLatencyNs, current, event.DurationNs) {
					break
				}
			}

			// Log slow syscalls
			if event.DurationNs > 10*time.Millisecond.Nanoseconds() {
				m.logger.Warn("Very slow syscall detected",
					zap.String("comm", string(event.Comm[:bytes.IndexByte(event.Comm[:], 0)])),
					zap.Uint32("syscall", event.SyscallNr),
					zap.Duration("duration", time.Duration(event.DurationNs)),
				)
			}
		}
	}
}

// collectStats periodically collects and logs statistics
func (m *LinuxMonitor) collectStats(ctx context.Context) {
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
			count := atomic.LoadUint64(&m.slowSyscalls)
			if count == 0 {
				continue
			}

			total := atomic.LoadUint64(&m.totalLatencyNs)
			max := atomic.LoadUint64(&m.maxLatencyNs)
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
func (m *LinuxMonitor) GetStats() (slowSyscalls uint64, avgLatencyNs uint64, maxLatencyNs uint64) {
	count := atomic.LoadUint64(&m.slowSyscalls)
	if count == 0 {
		return 0, 0, 0
	}

	total := atomic.LoadUint64(&m.totalLatencyNs)
	return count, total / count, atomic.LoadUint64(&m.maxLatencyNs)
}
