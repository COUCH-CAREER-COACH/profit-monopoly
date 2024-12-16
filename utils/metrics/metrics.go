package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	registry = prometheus.NewRegistry()
	logger   *zap.Logger
)

type MetricsConfig struct {
	ReportInterval time.Duration
	LogMetrics     bool
}

func Initialize(cfg *MetricsConfig, log *zap.Logger) {
	logger = log
	prometheus.DefaultRegisterer = registry
}

type NetworkMetrics struct {
	Errors       prometheus.Counter
	BytesSent    prometheus.Counter
	BytesRecv    prometheus.Counter
	SendLatency  prometheus.Histogram
	RecvLatency  prometheus.Histogram
	Connections  prometheus.Counter
	Disconnects  prometheus.Counter
	Reconnects   prometheus.Counter
	RxPackets    prometheus.Counter
	TxPackets    prometheus.Counter
	TxCount      prometheus.Gauge
	TxEvictions  prometheus.Gauge
	TxLookups    prometheus.Gauge
	TxCollisions prometheus.Gauge
	GasPrice     prometheus.Histogram
}

func NewNetworkMetrics(namespace string) *NetworkMetrics {
	return &NetworkMetrics{
		Errors: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "errors_total",
			Help:      "Total number of network errors",
		}),
		BytesSent: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "bytes_sent_total",
			Help:      "Total number of bytes sent",
		}),
		BytesRecv: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "bytes_recv_total",
			Help:      "Total number of bytes received",
		}),
		SendLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "send_latency_seconds",
			Help:      "Send latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 10),
		}),
		RecvLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "recv_latency_seconds",
			Help:      "Receive latency in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 10),
		}),
		Connections: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connections_total",
			Help:      "Total number of connections",
		}),
		Disconnects: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "disconnects_total",
			Help:      "Total number of disconnections",
		}),
		Reconnects: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "reconnects_total",
			Help:      "Total number of reconnection attempts",
		}),
		RxPackets: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "rx_packets_total",
			Help:      "Total number of received packets",
		}),
		TxPackets: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "tx_packets_total",
			Help:      "Total number of transmitted packets",
		}),
		TxCount: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "tx_count",
			Help:      "Current number of transactions",
		}),
		TxEvictions: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "tx_evictions",
			Help:      "Number of transaction evictions",
		}),
		TxLookups: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "tx_lookups",
			Help:      "Number of transaction lookups",
		}),
		TxCollisions: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "tx_collisions",
			Help:      "Number of transaction collisions",
		}),
		GasPrice: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "gas_price",
			Help:      "Gas price distribution",
			Buckets:   prometheus.ExponentialBuckets(1e9, 2, 15), // Start at 1 gwei
		}),
	}
}

type MempoolMetrics struct {
	TxCount        prometheus.Counter
	TxSize         prometheus.Histogram
	GasPrice       prometheus.Histogram
	ProcessTime    prometheus.Histogram
	DroppedTx      prometheus.Counter
	InvalidTx      prometheus.Counter
	ProfitableTx   prometheus.Counter
	UnprofitableTx prometheus.Counter
}

func NewMempoolMetrics(namespace string) *MempoolMetrics {
	return &MempoolMetrics{
		TxCount: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "transactions_total",
			Help:      "Total number of transactions processed",
		}),
		TxSize: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "transaction_size_bytes",
			Help:      "Transaction size in bytes",
			Buckets:   prometheus.ExponentialBuckets(256, 2, 10),
		}),
		GasPrice: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "gas_price",
			Help:      "Gas price",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 10),
		}),
		ProcessTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "process_time_seconds",
			Help:      "Time taken to process transactions",
			Buckets:   prometheus.DefBuckets,
		}),
		DroppedTx: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dropped_transactions_total",
			Help:      "Total number of dropped transactions",
		}),
		InvalidTx: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "invalid_transactions_total",
			Help:      "Total number of invalid transactions",
		}),
		ProfitableTx: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "profitable_transactions_total",
			Help:      "Total number of profitable transactions",
		}),
		UnprofitableTx: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "unprofitable_transactions_total",
			Help:      "Total number of unprofitable transactions",
		}),
	}
}

type SystemMetrics struct {
	CPUUsage    prometheus.Gauge
	MemoryUsage prometheus.Gauge
	DiskIO      prometheus.Counter
	NetworkIO   prometheus.Counter
	GCPause     prometheus.Histogram
	Goroutines  prometheus.Gauge
	Uptime      prometheus.Counter
}

func NewSystemMetrics(namespace string) *SystemMetrics {
	return &SystemMetrics{
		CPUUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cpu_usage_percent",
			Help:      "CPU usage percentage",
		}),
		MemoryUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "memory_usage_bytes",
			Help:      "Memory usage in bytes",
		}),
		DiskIO: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "disk_io_bytes_total",
			Help:      "Total disk I/O in bytes",
		}),
		NetworkIO: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "network_io_bytes_total",
			Help:      "Total network I/O in bytes",
		}),
		GCPause: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "gc_pause_seconds",
			Help:      "GC pause time in seconds",
			Buckets:   prometheus.DefBuckets,
		}),
		Goroutines: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "goroutines",
			Help:      "Number of goroutines",
		}),
		Uptime: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "uptime_seconds",
			Help:      "Uptime in seconds",
		}),
	}
}

type StrategyMetrics struct {
	Attempts      prometheus.Counter
	Successes     prometheus.Counter
	Failures      prometheus.Counter
	ProfitTotal   prometheus.Counter
	GasUsed       prometheus.Histogram
	ExecutionTime prometheus.Histogram
	RevertRate    prometheus.Counter
	Frontrunning  prometheus.Counter
	Backrunning   prometheus.Counter
	Sandwiching   prometheus.Counter
}

func NewStrategyMetrics(namespace string) *StrategyMetrics {
	return &StrategyMetrics{
		Attempts: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "attempts_total",
			Help:      "Total number of strategy attempts",
		}),
		Successes: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "successes_total",
			Help:      "Total number of successful strategy executions",
		}),
		Failures: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "failures_total",
			Help:      "Total number of failed strategy executions",
		}),
		ProfitTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "profit_total_wei",
			Help:      "Total profit in wei",
		}),
		GasUsed: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "gas_used",
			Help:      "Gas used per strategy execution",
			Buckets:   prometheus.ExponentialBuckets(21000, 2, 10),
		}),
		ExecutionTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "execution_time_seconds",
			Help:      "Time taken to execute strategy",
			Buckets:   prometheus.DefBuckets,
		}),
		RevertRate: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "revert_rate",
			Help:      "Rate of transaction reverts",
		}),
		Frontrunning: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "frontrunning_total",
			Help:      "Total number of frontrunning attempts",
		}),
		Backrunning: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "backrunning_total",
			Help:      "Total number of backrunning attempts",
		}),
		Sandwiching: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sandwiching_total",
			Help:      "Total number of sandwiching attempts",
		}),
	}
}
