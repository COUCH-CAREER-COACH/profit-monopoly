package config

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
)

type Config struct {
	// Chain and network settings
	ChainID      uint64 `json:"chain_id"`
	RPCEndpoint  string `json:"rpc_endpoint"`
	WSEndpoint   string `json:"ws_endpoint"`
	FlashbotsRPC string `json:"flashbots_rpc"`

	// Monitoring intervals
	BlockMonitorInterval   time.Duration `json:"block_monitor_interval"`
	MetricsUpdateInterval  time.Duration `json:"metrics_update_interval"`
	HealthCheckInterval    time.Duration `json:"health_check_interval"`
	BuilderUpdateInterval  time.Duration `json:"builder_update_interval"`
	PredictionUpdatePeriod time.Duration `json:"prediction_update_period"`
	ResourceCheckFrequency time.Duration `json:"resource_check_frequency"`
	ErrorAnalysisFrequency time.Duration `json:"error_analysis_frequency"`

	// Performance thresholds
	MinProfitThreshold   *big.Int `json:"min_profit_threshold"`
	MaxGasPrice          *big.Int `json:"max_gas_price"`
	MaxPendingTxns       int      `json:"max_pending_txns"`
	MaxConcurrentBundles int      `json:"max_concurrent_bundles"`
	ResourceUsageLimit   float64  `json:"resource_usage_limit"`
	ErrorRateThreshold   float64  `json:"error_rate_threshold"`

	// Network settings
	NetworkTimeout   time.Duration `json:"network_timeout"`
	KeepAlive        time.Duration `json:"keep_alive"`
	ReconnectBackoff time.Duration `json:"reconnect_backoff"`
	MaxReconnects    int           `json:"max_reconnects"`
	MetricsInterval  time.Duration `json:"metrics_interval"`
	ReadBufferSize   int           `json:"read_buffer_size"`
	WriteBufferSize  int           `json:"write_buffer_size"`
	CPUAffinity      []int         `json:"cpu_affinity"`

	// System configuration
	System                SystemConfig         `json:"system"`
	CircuitBreaker        CircuitBreakerConfig `json:"circuit_breaker"`
	RPCRateLimit          RateLimitConfig      `json:"rpc_rate_limit"`
	FlashbotsRateLimit    RateLimitConfig      `json:"flashbots_rate_limit"`
	BlockBuilderRateLimit RateLimitConfig      `json:"block_builder_rate_limit"`

	// Feature flags
	PrometheusEnabled  bool   `json:"prometheus_enabled"`
	PrometheusEndpoint string `json:"prometheus_endpoint"`
	AlertingEnabled    bool   `json:"alerting_enabled"`
	AlertingEndpoint   string `json:"alerting_endpoint"`

	// DPDK configuration
	DPDKConfig *DPDKConfig `json:"dpdk_config,omitempty"`

	// Network configuration
	Network NetworkConfig `json:"network"`

	// Internal components
	Logger *zap.Logger `json:"-"`
}

type PoolConfig struct {
	Address  common.Address `json:"address"`
	Token0   common.Address `json:"token0"`
	Token1   common.Address `json:"token1"`
	Fee      uint32         `json:"fee"`
	Reserve0 *big.Int       `json:"reserve0"`
	Reserve1 *big.Int       `json:"reserve1"`
}

type ChainConfig struct {
	ChainID            uint64        `json:"chain_id"`
	MinConfirmations   uint64        `json:"min_confirmations"`
	BlockTime          time.Duration `json:"block_time"`
	GasLimit           uint64        `json:"gas_limit"`
	MaxGasPrice        *big.Int      `json:"max_gas_price"`
	FlashLoanProviders []string      `json:"flash_loan_providers"`
	SupportedDEXes     []string      `json:"supported_dexes"`
	BlacklistedTokens  []string      `json:"blacklisted_tokens"`
	RPCEndpoint        string        `json:"rpc_endpoint"`
	WSEndpoint         string        `json:"ws_endpoint"`
	FlashbotsRPC       string        `json:"flashbots_rpc"`
	FlashbotsKey       string        `json:"flashbots_key"`
	BlockBuilderURL    string        `json:"block_builder_url"`
}

type SystemConfig struct {
	// CPU and Memory settings
	CPUPinning     []int  `json:"cpu_pinning"`      // List of CPU cores to pin critical threads to
	HugePagesCount int    `json:"huge_pages_count"` // Number of huge pages to allocate
	HugePageSize   string `json:"huge_page_size"`   // Size of huge pages (e.g., "2MB" or "1GB")
	MemoryLimit    string `json:"memory_limit"`     // Maximum memory limit for the process

	// DPDK settings
	DPDKEnabled     bool     `json:"dpdk_enabled"`      // Enable DPDK for network optimization
	DPDKPorts       []uint16 `json:"dpdk_ports"`        // DPDK port numbers to use
	DPDKMemChannels int      `json:"dpdk_mem_channels"` // Number of memory channels

	// eBPF settings
	EBPFEnabled     bool   `json:"ebpf_enabled"`      // Enable eBPF optimizations
	EBPFProgramPath string `json:"ebpf_program_path"` // Path to eBPF program
}

type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled"`
	ErrorThreshold   int           `json:"error_threshold"`
	ResetInterval    time.Duration `json:"reset_interval"`
	CooldownPeriod   time.Duration `json:"cooldown_period"`
	MinHealthyPeriod time.Duration `json:"min_healthy_period"`
}

type RateLimitConfig struct {
	RequestsPerSecond float64       `json:"requests_per_second"`
	BurstSize         int           `json:"burst_size"`
	WaitTimeout       time.Duration `json:"wait_timeout"`
}

type DPDKConfig struct {
	Enabled        bool
	MemoryChannels int
	MemorySize     int // in MB
	InterfaceName  string
	RXQueueSize    int
	TXQueueSize    int
	NumRXQueues    int
	NumTXQueues    int
	HugePageDir    string
	PCIWhitelist   []string
	PCIBlacklist   []string
	LogLevel       string
	SocketMemory   string
	CPUAllowList   string
}

type NetworkConfig struct {
	HTTPEndpoint   string
	WSEndpoint     string
	ChainID        int64
	FlashbotsRelay string
}

type SecureConfig struct {
	PrivateKey    string
	FlashbotsKey  string
}

func (c *Config) ValidateConfig() error {
	var errors []string

	// Validate Chain and Network settings
	if c.ChainID == 0 {
		errors = append(errors, "chain_id must be specified")
	}
	if c.RPCEndpoint == "" {
		errors = append(errors, "rpc_endpoint must be specified")
	}
	if c.WSEndpoint == "" {
		errors = append(errors, "ws_endpoint must be specified")
	}

	// Validate Performance Thresholds
	if c.MinProfitThreshold == nil || c.MinProfitThreshold.Sign() <= 0 {
		errors = append(errors, "min_profit_threshold must be positive")
	}
	if c.MaxGasPrice == nil || c.MaxGasPrice.Sign() <= 0 {
		errors = append(errors, "max_gas_price must be positive")
	}
	if c.MaxPendingTxns <= 0 {
		errors = append(errors, "max_pending_txns must be positive")
	}
	if c.MaxConcurrentBundles <= 0 {
		errors = append(errors, "max_concurrent_bundles must be positive")
	}

	// Validate System Configuration
	if err := c.System.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("system config error: %v", err))
	}

	// Validate Circuit Breaker
	if err := c.CircuitBreaker.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("circuit breaker error: %v", err))
	}

	// Validate Rate Limits
	if err := c.RPCRateLimit.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("RPC rate limit error: %v", err))
	}
	if err := c.FlashbotsRateLimit.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("Flashbots rate limit error: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

func (s *SystemConfig) Validate() error {
	if s.DPDKEnabled {
		if len(s.DPDKPorts) == 0 {
			return fmt.Errorf("DPDK ports must be specified when DPDK is enabled")
		}
		if s.DPDKMemChannels <= 0 {
			return fmt.Errorf("DPDK memory channels must be positive")
		}
	}

	if s.EBPFEnabled && s.EBPFProgramPath == "" {
		return fmt.Errorf("eBPF program path must be specified when eBPF is enabled")
	}

	return nil
}

func (c *CircuitBreakerConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.ErrorThreshold <= 0 {
		return fmt.Errorf("error threshold must be positive")
	}
	if c.ResetInterval <= 0 {
		return fmt.Errorf("reset interval must be positive")
	}
	if c.CooldownPeriod <= 0 {
		return fmt.Errorf("cooldown period must be positive")
	}
	if c.MinHealthyPeriod <= 0 {
		return fmt.Errorf("minimum healthy period must be positive")
	}

	return nil
}

func (r *RateLimitConfig) Validate() error {
	if r.RequestsPerSecond <= 0 {
		return fmt.Errorf("requests per second must be positive")
	}
	if r.BurstSize <= 0 {
		return fmt.Errorf("burst size must be positive")
	}
	if r.WaitTimeout <= 0 {
		return fmt.Errorf("wait timeout must be positive")
	}

	return nil
}

func LoadConfig(cfgFile string) (*Config, error) {
	if cfgFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		cfgFile = filepath.Join(home, ".mevbot.json")
	}

	file, err := os.Open(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	config.Logger = logger

	// Validate configuration
	if err := config.ValidateConfig(); err != nil {
		return nil, err
	}

	return &config, nil
}

func LoadSecureConfig() (*SecureConfig, error) {
	privateKey, err := GetRequiredEnv("MEV_BOT_PRIVATE_KEY")
	if err != nil {
		return nil, fmt.Errorf("private key not found: %w", err)
	}

	flashbotsKey, err := GetRequiredEnv("FLASHBOTS_KEY")
	if err != nil {
		return nil, fmt.Errorf("flashbots key not found: %w", err)
	}

	return &SecureConfig{
		PrivateKey:    privateKey,
		FlashbotsKey:  flashbotsKey,
	}, nil
}

func SaveConfig(cfg *Config, cfgFile string) error {
	if cfgFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		cfgFile = filepath.Join(home, ".mevbot.json")
	}

	file, err := os.Create(cfgFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	return encoder.Encode(cfg)
}

func NewConfig() (*Config, error) {
	httpEndpoint := "http://localhost:8545"
	wsEndpoint := "ws://localhost:8546"
	chainID := "1" // default to mainnet
	chainIDInt, err := strconv.ParseUint(chainID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chain ID: %w", err)
	}

	return &Config{
		ChainID:                chainIDInt,
		RPCEndpoint:            httpEndpoint,
		WSEndpoint:             wsEndpoint,
		BlockMonitorInterval:   time.Second * 1,
		MetricsUpdateInterval:  time.Second * 10,
		HealthCheckInterval:    time.Second * 30,
		BuilderUpdateInterval:  time.Second * 60,
		PredictionUpdatePeriod: time.Minute * 5,
		ResourceCheckFrequency: time.Second * 5,
		ErrorAnalysisFrequency: time.Minute,
		MinProfitThreshold:     big.NewInt(100000000000000000), // 0.1 ETH
		MaxGasPrice:            big.NewInt(500000000000),       // 500 Gwei
		MaxPendingTxns:         100,
		MaxConcurrentBundles:   10,
		ResourceUsageLimit:     0.9,
		ErrorRateThreshold:     0.05,
		NetworkTimeout:         5 * time.Second,
		KeepAlive:              30 * time.Second,
		ReconnectBackoff:       1 * time.Second,
		MaxReconnects:          3,
		MetricsInterval:        10 * time.Second,
		ReadBufferSize:         4096,
		WriteBufferSize:        4096,
		CPUAffinity:            []int{0, 1}, // Default to first two CPU cores
		System: SystemConfig{
			CPUPinning:      []int{0, 1},
			HugePagesCount:  1024,
			HugePageSize:    "2MB",
			MemoryLimit:     "16GB",
			DPDKEnabled:     false,
			DPDKPorts:       []uint16{0, 1},
			DPDKMemChannels: 4,
			EBPFEnabled:     false,
			EBPFProgramPath: "",
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          false,
			ErrorThreshold:   10,
			ResetInterval:    time.Minute,
			CooldownPeriod:   time.Second * 30,
			MinHealthyPeriod: time.Second * 10,
		},
		RPCRateLimit: RateLimitConfig{
			RequestsPerSecond: 10,
			BurstSize:         100,
			WaitTimeout:       time.Second,
		},
		FlashbotsRateLimit: RateLimitConfig{
			RequestsPerSecond: 10,
			BurstSize:         100,
			WaitTimeout:       time.Second,
		},
		BlockBuilderRateLimit: RateLimitConfig{
			RequestsPerSecond: 10,
			BurstSize:         100,
			WaitTimeout:       time.Second,
		},
		PrometheusEnabled:  false,
		PrometheusEndpoint: "",
		AlertingEnabled:    false,
		AlertingEndpoint:   "",
		DPDKConfig: &DPDKConfig{
			Enabled:        false,
			MemoryChannels: 4,
			MemorySize:     1024,
			InterfaceName:  "eth0",
			RXQueueSize:    1024,
			TXQueueSize:    1024,
			NumRXQueues:    1,
			NumTXQueues:    1,
			HugePageDir:    "/dev/hugepages",
			PCIWhitelist:   []string{},
			PCIBlacklist:   []string{},
			LogLevel:       "notice",
			SocketMemory:   "1024",
			CPUAllowList:   "0-3",
		},
		Network: NetworkConfig{
			HTTPEndpoint:   "http://localhost:8545",
			WSEndpoint:     "ws://localhost:8546",
			ChainID:        1,
			FlashbotsRelay: "https://relay.flashbots.net",
		},
	}, nil
}

func DefaultConfig() *Config {
	return &Config{
		Logger:                 zap.NewNop(),
		BlockMonitorInterval:   time.Second * 1,
		MetricsUpdateInterval:  time.Second * 10,
		HealthCheckInterval:    time.Second * 30,
		BuilderUpdateInterval:  time.Second * 60,
		PredictionUpdatePeriod: time.Minute * 5,
		ResourceCheckFrequency: time.Second * 5,
		ErrorAnalysisFrequency: time.Minute,
		MinProfitThreshold:     big.NewInt(100000000000000000), // 0.1 ETH
		MaxGasPrice:            big.NewInt(500000000000),       // 500 Gwei
		MaxPendingTxns:         100,
		MaxConcurrentBundles:   10,
		ResourceUsageLimit:     0.9,
		ErrorRateThreshold:     0.05,
		NetworkTimeout:         5 * time.Second,
		KeepAlive:              30 * time.Second,
		ReconnectBackoff:       1 * time.Second,
		MaxReconnects:          3,
		MetricsInterval:        10 * time.Second,
		ReadBufferSize:         4096,
		WriteBufferSize:        4096,
		CPUAffinity:            []int{0, 1}, // Default to first two CPU cores
		System: SystemConfig{
			CPUPinning:      []int{0, 1},
			HugePagesCount:  1024,
			HugePageSize:    "2MB",
			MemoryLimit:     "16GB",
			DPDKEnabled:     false,
			DPDKPorts:       []uint16{0, 1},
			DPDKMemChannels: 4,
			EBPFEnabled:     false,
			EBPFProgramPath: "",
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          false,
			ErrorThreshold:   10,
			ResetInterval:    time.Minute,
			CooldownPeriod:   time.Second * 30,
			MinHealthyPeriod: time.Second * 10,
		},
		RPCRateLimit: RateLimitConfig{
			RequestsPerSecond: 10,
			BurstSize:         100,
			WaitTimeout:       time.Second,
		},
		FlashbotsRateLimit: RateLimitConfig{
			RequestsPerSecond: 10,
			BurstSize:         100,
			WaitTimeout:       time.Second,
		},
		BlockBuilderRateLimit: RateLimitConfig{
			RequestsPerSecond: 10,
			BurstSize:         100,
			WaitTimeout:       time.Second,
		},
		PrometheusEnabled:  false,
		PrometheusEndpoint: "",
		AlertingEnabled:    false,
		AlertingEndpoint:   "",
		DPDKConfig: &DPDKConfig{
			Enabled:        false,
			MemoryChannels: 4,
			MemorySize:     1024,
			InterfaceName:  "eth0",
			RXQueueSize:    1024,
			TXQueueSize:    1024,
			NumRXQueues:    1,
			NumTXQueues:    1,
			HugePageDir:    "/dev/hugepages",
			PCIWhitelist:   []string{},
			PCIBlacklist:   []string{},
			LogLevel:       "notice",
			SocketMemory:   "1024",
			CPUAllowList:   "0-3",
		},
		Network: NetworkConfig{
			HTTPEndpoint:   "http://localhost:8545",
			WSEndpoint:     "ws://localhost:8546",
			ChainID:        1,
			FlashbotsRelay: "https://relay.flashbots.net",
		},
	}
}

func getEnvWithDefault(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvArrayWithDefault(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func GetRequiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("required environment variable %s not set", key)
	}
	return value, nil
}

func GetNetworkEndpoints() (string, string, error) {
	network := getEnvWithDefault("NETWORK", "mainnet")
	infuraKey, err := GetRequiredEnv("INFURA_KEY")
	if err != nil {
		return "", "", err
	}

	var httpEndpoint, wsEndpoint string
	switch network {
	case "mainnet":
		httpEndpoint = fmt.Sprintf("https://mainnet.infura.io/v3/%s", infuraKey)
		wsEndpoint = fmt.Sprintf("wss://mainnet.infura.io/ws/v3/%s", infuraKey)
	case "sepolia":
		httpEndpoint = fmt.Sprintf("https://sepolia.infura.io/v3/%s", infuraKey)
		wsEndpoint = fmt.Sprintf("wss://sepolia.infura.io/ws/v3/%s", infuraKey)
	case "holesky":
		httpEndpoint = fmt.Sprintf("https://holesky.infura.io/v3/%s", infuraKey)
		wsEndpoint = fmt.Sprintf("wss://holesky.infura.io/ws/v3/%s", infuraKey)
	default:
		return "", "", fmt.Errorf("unsupported network: %s", network)
	}

	return httpEndpoint, wsEndpoint, nil
}

func LoadNetworkConfig() (*NetworkConfig, error) {
	httpEndpoint, wsEndpoint, err := GetNetworkEndpoints()
	if err != nil {
		return nil, fmt.Errorf("failed to get network endpoints: %w", err)
	}

	return &NetworkConfig{
		HTTPEndpoint:   httpEndpoint,
		WSEndpoint:     wsEndpoint,
		FlashbotsRelay: getEnvWithDefault("FLASHBOTS_RELAY", "https://relay.flashbots.net"),
	}, nil
}
