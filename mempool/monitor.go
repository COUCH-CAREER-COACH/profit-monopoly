package mempool

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/michaelpento.lv/mevbot/config"
	"github.com/michaelpento.lv/mevbot/mempool/network/dpdk"
)

// MempoolMonitor manages transaction monitoring and analysis
type MempoolMonitor struct {
	cfg        *config.Config
	client     *ethclient.Client
	logger     *zap.Logger
	txChan     chan *Transaction
	txPool     *MMapPool
	txIndexer  *MempoolIndexer
	workers    *AffinityWorkerPool
	limiter    *rate.Limiter
	cache      *lru.Cache
	breaker    *CircuitBreaker
	netManager *dpdk.DPDKManager
	metrics    struct {
		transactionCount  *prometheus.CounterVec
		blockCount       *prometheus.CounterVec
		txLatency        prometheus.Histogram
		profitTotal      prometheus.Counter
		profitPerTx      prometheus.Histogram
		memoryUsage      prometheus.Gauge
		goroutineCount   prometheus.Gauge
		opportunityCount *prometheus.CounterVec
		errorCount       prometheus.Counter
		queueDepth       prometheus.Gauge
	}
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewMempoolMonitor creates a new mempool monitor
func NewMempoolMonitor(cfg *config.Config, client *ethclient.Client, logger *zap.Logger) (*MempoolMonitor, error) {
	// Create memory-mapped transaction pool
	txPool, err := NewMMapPool(cfg.MempoolConfig.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create mmap pool: %w", err)
	}

	// Create worker pool for parallel processing
	workers := NewAffinityWorkerPool(&WorkerConfig{
		NumWorkers: runtime.NumCPU(),
		QueueSize: 10000, // Large queue for burst handling
	})

	// Create DPDK network manager
	netManager, err := dpdk.NewDPDKManager(logger, &dpdk.Config{
		Interface:    "eth0",  // Main network interface
		NumRxQueues: 4,       // Number of RX queues
		NumTxQueues: 4,       // Number of TX queues
	})
	if err != nil {
		txPool.Close()
		return nil, fmt.Errorf("failed to create DPDK manager: %w", err)
	}

	// Create LRU cache for transaction deduplication
	cache, err := lru.New(10000)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	m := &MempoolMonitor{
		cfg:        cfg,
		client:     client,
		logger:     logger,
		txChan:     make(chan *Transaction, 10000),
		txPool:     txPool,
		workers:    workers,
		netManager: netManager,
		limiter:    rate.NewLimiter(rate.Limit(cfg.RateLimit), cfg.RateBurst),
		breaker:    NewCircuitBreaker(cfg.CircuitBreaker, logger),
		cache:      cache,
	}

	m.initMetrics()
	return m, nil
}

// Start starts the mempool monitor
func (m *MempoolMonitor) Start(ctx context.Context) chan *Transaction {
	m.ctx, m.cancel = context.WithCancel(ctx)

	// Start worker pool
	if err := m.workers.Start(ctx); err != nil {
		m.logger.Error("Failed to start worker pool", zap.Error(err))
		return nil
	}

	// Start DPDK network manager
	if err := m.netManager.Start(ctx); err != nil {
		m.logger.Error("Failed to start DPDK manager", zap.Error(err))
		m.workers.Stop()
		return nil
	}

	// Start monitoring loops
	m.wg.Add(2)
	go m.monitorLoop(ctx)
	go m.collectSystemMetrics(ctx)

	return m.txChan
}

// Shutdown gracefully shuts down the mempool monitor
func (m *MempoolMonitor) Shutdown() error {
	if m.cancel != nil {
		m.cancel()
	}

	m.wg.Wait()

	// Stop worker pool
	if err := m.workers.Stop(); err != nil {
		m.logger.Error("Failed to stop worker pool", zap.Error(err))
	}

	// Stop DPDK manager
	if err := m.netManager.Stop(); err != nil {
		m.logger.Error("Failed to stop DPDK manager", zap.Error(err))
	}

	// Close transaction pool
	if err := m.txPool.Close(); err != nil {
		m.logger.Error("Failed to close transaction pool", zap.Error(err))
	}

	return nil
}

func (m *MempoolMonitor) monitorLoop(ctx context.Context) {
	defer m.wg.Done()

	const (
		maxRetries    = 3
		retryInterval = 5 * time.Second
		batchSize     = 100
		batchTimeout  = 50 * time.Millisecond
	)

	for retries := 0; retries < maxRetries; retries++ {
		// Subscribe to pending transactions
		pendingTxHashes := make(chan common.Hash, 1024)
		sub, err := m.client.SubscribePendingTransactions(ctx, pendingTxHashes)
		if err != nil {
			m.logger.Error("Failed to subscribe to pending transactions",
				zap.Error(err),
				zap.Int("retry", retries+1),
			)
			m.metrics.errorCount.Inc()

			select {
			case <-ctx.Done():
				return
			case <-time.After(retryInterval):
				continue
			}
		}
		defer sub.Unsubscribe()

		// Reset retry counter on successful subscription
		retries = 0

		// Create batch processor
		batch := make([]*Transaction, 0, batchSize)
		batchTimer := time.NewTimer(batchTimeout)
		defer batchTimer.Stop()

		// Process pending transactions
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-sub.Err():
				m.logger.Error("Subscription error", zap.Error(err))
				m.breaker.RecordError(err)
				m.metrics.errorCount.Inc()
				// Break inner loop to retry subscription
				goto RETRY
			case hash := <-pendingTxHashes:
				// Fetch full transaction
				tx, err := m.client.TransactionByHash(ctx, hash)
				if err != nil {
					m.logger.Error("Failed to get transaction",
						zap.Error(err),
						zap.String("hash", hash.Hex()),
					)
					m.metrics.errorCount.Inc()
					continue
				}

				// Create transaction object
				transaction := &Transaction{
					Transaction: tx,
					FirstSeen:  time.Now(),
					GasPrice:   tx.GasPrice(),
					Priority:   calculatePriority(tx),
				}

				// Add to batch
				batch = append(batch, transaction)

				// Process batch if full
				if len(batch) >= batchSize {
					m.processBatch(batch)
					batch = make([]*Transaction, 0, batchSize)
					batchTimer.Reset(batchTimeout)
				}

			case <-batchTimer.C:
				// Process remaining transactions in batch
				if len(batch) > 0 {
					m.processBatch(batch)
					batch = make([]*Transaction, 0, batchSize)
				}
				batchTimer.Reset(batchTimeout)
			}
		}
	RETRY:
		time.Sleep(retryInterval)
		continue
	}

	m.logger.Error("Max retries exceeded for subscription")
}

func (m *MempoolMonitor) processBatch(batch []*Transaction) {
	if len(batch) == 0 {
		return
	}

	start := time.Now()
	m.metrics.queueDepth.Set(float64(len(batch)))

	for _, tx := range batch {
		if err := m.processPendingTx(tx); err != nil {
			m.logger.Error("Failed to process transaction",
				zap.Error(err),
				zap.String("hash", tx.Hash().Hex()),
			)
			m.metrics.errorCount.Inc()
		}
	}

	m.metrics.transactionCount.WithLabelValues("processed").Add(float64(len(batch)))
	m.metrics.txLatency.Observe(float64(time.Since(start).Milliseconds()))
}

func (m *MempoolMonitor) processPendingTx(tx *Transaction) error {
	if !m.isRelevantTransaction(tx) {
		return nil
	}

	if !m.breaker.IsHealthy() {
		return errors.New("circuit breaker is tripped")
	}

	if err := m.limiter.Wait(m.ctx); err != nil {
		return fmt.Errorf("rate limiter error: %w", err)
	}

	// Record metrics
	m.metrics.transactionCount.WithLabelValues("processed").Inc()
	start := time.Now()
	defer func() {
		m.metrics.txLatency.Observe(float64(time.Since(start).Milliseconds()))
	}()

	// Process transaction
	if err := m.txIndexer.IndexTransaction(tx); err != nil {
		m.metrics.errorCount.Inc()
		return fmt.Errorf("failed to index transaction: %w", err)
	}

	return nil
}

func (m *MempoolMonitor) initMetrics() {
	m.metrics.transactionCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mempool_transactions_total",
			Help: "Total number of transactions processed by type",
		},
		[]string{"type"},
	)

	m.metrics.blockCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mempool_blocks_total",
			Help: "Total number of blocks processed",
		},
		[]string{"type"},
	)

	m.metrics.txLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "mempool_transaction_latency_seconds",
		Help:    "Histogram of transaction processing latencies",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // Start at 1ms, double 10 times
	})

	m.metrics.profitTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mempool_profit_total_wei",
		Help: "Total profit in wei",
	})

	m.metrics.profitPerTx = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "mempool_profit_per_tx_wei",
		Help:    "Histogram of profit per transaction in wei",
		Buckets: prometheus.ExponentialBuckets(1e9, 2, 15), // Start at 1 Gwei
	})

	m.metrics.memoryUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mempool_memory_usage_bytes",
		Help: "Current memory usage in bytes",
	})

	m.metrics.goroutineCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mempool_goroutine_count",
		Help: "Current number of goroutines",
	})

	m.metrics.opportunityCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mempool_opportunities_total",
			Help: "Total number of MEV opportunities by type",
		},
		[]string{"type"},
	)

	m.metrics.errorCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mempool_errors_total",
		Help: "Total number of errors encountered",
	})

	m.metrics.queueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mempool_queue_depth",
		Help: "Current number of transactions in processing queue",
	})

	// Register metrics
	prometheus.MustRegister(
		m.metrics.transactionCount,
		m.metrics.blockCount,
		m.metrics.txLatency,
		m.metrics.profitTotal,
		m.metrics.profitPerTx,
		m.metrics.memoryUsage,
		m.metrics.goroutineCount,
		m.metrics.opportunityCount,
		m.metrics.errorCount,
		m.metrics.queueDepth,
	)
}

func (m *MempoolMonitor) collectSystemMetrics(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Collect network statistics
			rxPkts, txPkts, rxBytes, txBytes, rxDrops, txDrops, netLatency := m.netManager.GetStats()

			m.logger.Info("System metrics",
				// Network metrics
				zap.Uint64("rx_packets", rxPkts),
				zap.Uint64("tx_packets", txPkts),
				zap.Uint64("rx_bytes", rxBytes),
				zap.Uint64("tx_bytes", txBytes),
				zap.Uint64("rx_drops", rxDrops),
				zap.Uint64("tx_drops", txDrops),
				zap.Duration("network_latency", time.Duration(netLatency)),
			)

			// Other metrics collection...
			m.metrics.memoryUsage.Set(float64(m.getMemoryUsage()))
			m.metrics.goroutineCount.Set(float64(runtime.NumGoroutine()))
		}
	}
}

func (m *MempoolMonitor) getMemoryUsage() uint64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.Alloc
}

func (m *MempoolMonitor) isRelevantTransaction(tx *Transaction) bool {
	if tx == nil {
		return false
	}

	// Check gas price
	if tx.GasPrice.Cmp(m.cfg.MaxGasPrice) > 0 {
		return false
	}

	// Check if transaction is already processed
	if m.cache.Contains(tx.Hash()) {
		return false
	}

	return true
}

type Transaction struct {
	*types.Transaction
	FirstSeen time.Time
	GasPrice  *big.Int
	Priority  float64
}

type WorkerConfig struct {
	NumWorkers int
	QueueSize  int
}

type AffinityWorkerPool struct {
	workers int
	queue   chan func()
	wg      sync.WaitGroup
}

func NewAffinityWorkerPool(config *WorkerConfig) *AffinityWorkerPool {
	return &AffinityWorkerPool{
		workers: config.NumWorkers,
		queue:   make(chan func(), config.QueueSize),
	}
}

func (p *AffinityWorkerPool) Start(ctx context.Context) error {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx)
	}
	return nil
}

func (p *AffinityWorkerPool) Stop() error {
	close(p.queue)
	p.wg.Wait()
	return nil
}

func (p *AffinityWorkerPool) Submit(task func()) {
	p.queue <- task
}

func (p *AffinityWorkerPool) worker(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-p.queue:
			if !ok {
				return
			}
			task()
		}
	}
}

type MMapPool struct {
	// ...
}

func NewMMapPool(path string) (*MMapPool, error) {
	// ...
}

func (m *MMapPool) Close() error {
	// ...
}

type CircuitBreaker struct {
	config      *config.CircuitBreakerConfig
	errorCount  atomic.Uint64
	lastReset   time.Time
	lastTripped time.Time
	enabled     atomic.Bool
	healthy     atomic.Bool
	mu          sync.RWMutex
	logger      *zap.Logger
	metrics     struct {
		tripCount    prometheus.Counter
		errorRate    prometheus.Counter
		healthyTime  prometheus.Gauge
		lastTripTime prometheus.Gauge
	}
}

func NewCircuitBreaker(cfg *config.CircuitBreakerConfig, logger *zap.Logger) *CircuitBreaker {
	cb := &CircuitBreaker{
		config:    cfg,
		logger:    logger,
		lastReset: time.Now(),
		enabled:   atomic.Bool{},
		healthy:   atomic.Bool{},
	}

	cb.metrics.tripCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "circuit_breaker_trips_total",
		Help: "Total number of circuit breaker trips",
	})

	cb.metrics.errorRate = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "circuit_breaker_errors_total",
		Help: "Total number of errors recorded by circuit breaker",
	})

	cb.metrics.healthyTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "circuit_breaker_healthy_time_seconds",
		Help: "Time since last healthy state in seconds",
	})

	cb.metrics.lastTripTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "circuit_breaker_last_trip_time_seconds",
		Help: "Time since last circuit breaker trip in seconds",
	})

	prometheus.MustRegister(cb.metrics.tripCount)
	prometheus.MustRegister(cb.metrics.errorRate)
	prometheus.MustRegister(cb.metrics.healthyTime)
	prometheus.MustRegister(cb.metrics.lastTripTime)

	cb.enabled.Store(true)
	cb.healthy.Store(true)

	go cb.monitor()

	return cb
}

func (cb *CircuitBreaker) RecordError(err error) bool {
	if !cb.enabled.Load() {
		return false
	}

	newCount := cb.errorCount.Add(1)

	// Check if we've exceeded the error threshold
	if int(newCount) >= cb.config.ErrorThreshold {
		cb.tripCircuit()
		return true
	}

	return false
}

func (cb *CircuitBreaker) tripCircuit() {
	cb.enabled.Store(false) // Set to false when tripped
	cb.healthy.Store(false)
	cb.lastTripped = time.Now()
	cb.metrics.tripCount.Inc()
	cb.metrics.lastTripTime.Set(time.Since(cb.lastTripped).Seconds())

	cb.logger.Warn("Circuit breaker tripped",
		zap.Uint64("error_count", cb.errorCount.Load()),
		zap.Duration("since_last_trip", time.Since(cb.lastTripped)))
}

func (cb *CircuitBreaker) monitor() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cb.mu.Lock()

		// Check if we should reset
		if time.Since(cb.lastReset) >= cb.config.ResetInterval {
			cb.errorCount.Store(0)
			cb.lastReset = time.Now()

			// If we've been in cooldown for long enough, reset the circuit breaker
			if !cb.enabled.Load() && time.Since(cb.lastTripped) >= cb.config.CooldownPeriod {
				cb.enabled.Store(true) // Set to true when reset
				cb.healthy.Store(true)
				cb.logger.Info("Circuit breaker reset",
					zap.Duration("cooldown_period", cb.config.CooldownPeriod))
			}
		}

		// Update metrics
		if cb.healthy.Load() {
			cb.metrics.healthyTime.Set(time.Since(cb.lastTripped).Seconds())
		}

		cb.mu.Unlock()
	}
}

func (cb *CircuitBreaker) IsHealthy() bool {
	return cb.enabled.Load() && cb.healthy.Load()
}

func calculatePriority(tx *types.Transaction) float64 {
	// Basic priority calculation based on gas price
	// This will be enhanced with more sophisticated metrics
	return float64(tx.GasPrice().Uint64())
}
