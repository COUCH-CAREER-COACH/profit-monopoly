package dpdk

import (
	"net"
	"sync"
	"time"
)

const (
	// Connection tracking configuration
	maxConnections    = 100000 // Maximum number of tracked connections
	cleanupInterval   = time.Minute
	connectionTimeout = 5 * time.Minute
)

// connKey uniquely identifies a connection
type connKey struct {
	srcIP, dstIP     net.IP
	srcPort, dstPort uint16
}

// connInfo stores connection information
type connInfo struct {
	firstSeen time.Time
	lastSeen  time.Time
	rxPackets uint64
	txPackets uint64
	rxBytes   uint64
	txBytes   uint64
}

// connTracker tracks network connections
type connTracker struct {
	connections sync.Map    // Map of connKey -> *connInfo
	count       atomic.Int64
}

// newConnTracker creates a new connection tracker
func newConnTracker() *connTracker {
	ct := &connTracker{}

	// Start cleanup goroutine
	go ct.cleanup()

	return ct
}

// track tracks a connection and returns connection info
func (ct *connTracker) track(srcIP, dstIP net.IP, srcPort, dstPort uint16) *connInfo {
	key := connKey{
		srcIP:   srcIP,
		dstIP:   dstIP,
		srcPort: srcPort,
		dstPort: dstPort,
	}

	now := time.Now()

	// Try to load existing connection
	if v, ok := ct.connections.Load(key); ok {
		info := v.(*connInfo)
		info.lastSeen = now
		return info
	}

	// Check if we're at capacity
	if ct.count.Load() >= maxConnections {
		return nil
	}

	// Create new connection info
	info := &connInfo{
		firstSeen: now,
		lastSeen:  now,
	}

	// Store only if we're still under capacity (handle race condition)
	if actual, loaded := ct.connections.LoadOrStore(key, info); loaded {
		return actual.(*connInfo)
	}

	ct.count.Add(1)
	return info
}

// cleanup periodically removes stale connections
func (ct *connTracker) cleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		expired := make([]connKey, 0)

		// Find expired connections
		ct.connections.Range(func(k, v interface{}) bool {
			key := k.(connKey)
			info := v.(*connInfo)
			if now.Sub(info.lastSeen) > connectionTimeout {
				expired = append(expired, key)
			}
			return true
		})

		// Remove expired connections
		for _, key := range expired {
			ct.connections.Delete(key)
			ct.count.Add(-1)
		}
	}
}

// getStats returns connection tracking statistics
func (ct *connTracker) getStats() (activeConnections int64) {
	return ct.count.Load()
}
