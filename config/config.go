package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

// Config contains all configuration settings
type Config struct {
	// Network configuration
	Network struct {
		MainnetRPC     string   `json:"mainnet_rpc"`
		FlashbotsRelay string   `json:"flashbots_relay"`
		WSEndpoints    []string `json:"ws_endpoints"`
		ChainID        int64    `json:"chain_id"`
		RPCEndpoint    string   `json:"rpc_endpoint"`
		WSEndpoint     string   `json:"ws_endpoint"`
	} `json:"network"`

	// Mempool configuration
	MempoolConfig struct {
		MaxPendingTx       int     `json:"max_pending_tx"`
		BlockBufferSize    int     `json:"block_buffer_size"`
		MinProfitThreshold float64 `json:"min_profit_threshold"`
		GasBoostFactor    float64 `json:"gas_boost_factor"`
		DataDir           string  `json:"data_dir"`
		Workers           int     `json:"workers"`
	} `json:"mempool"`

	// Flash loan configuration
	FlashLoan struct {
		Providers       []string `json:"providers"`
		MaxLoanAmount  float64  `json:"max_loan_amount"`
		MinProfitRatio float64  `json:"min_profit_ratio"`
		MaxFlashLoanFee *big.Int `json:"max_flash_loan_fee"`
	} `json:"flashloan"`

	// Monitoring configuration
	Monitoring struct {
		PrometheusPort  int    `json:"prometheus_port"`
		HealthCheckPort int    `json:"health_check_port"`
		LogLevel        string `json:"log_level"`
	} `json:"monitoring"`

	// Security configuration
	Security struct {
		MaxSlippage             float64 `json:"max_slippage"`
		CircuitBreakerThreshold float64 `json:"circuit_breaker_threshold"`
		RateLimitPerSecond      int     `json:"rate_limit_per_second"`
	} `json:"security"`

	// Rate limiting
	RateLimit float64 `json:"rate_limit"`
	RateBurst int     `json:"rate_burst"`

	// Circuit breaker configuration
	CircuitBreaker *CircuitBreakerConfig

	// Pool configuration
	Pools []*PoolConfig `json:"pools"`

	// Router and executor addresses
	RouterAddress    string `json:"router_address"`
	SandwichExecutor string `json:"sandwich_executor"`

	// Gas configuration
	MaxGasPrice *big.Int `json:"max_gas_price"`

	// Flashbots configuration
	FlashbotsKey string `json:"flashbots_key"`
	FlashbotsRPC string `json:"flashbots_rpc"`

	// Client and logger (runtime fields)
	Client *ethclient.Client `json:"-"`
	Logger *zap.Logger      `json:"-"`
}

// PoolConfig contains pool-specific configuration
type PoolConfig struct {
	Address     string   `json:"address"`
	Token0      string   `json:"token0"`
	Token1      string   `json:"token1"`
	Fee         uint64   `json:"fee"`
	MinLiquidity *big.Int `json:"min_liquidity"`
}

// CircuitBreakerConfig contains circuit breaker settings
type CircuitBreakerConfig struct {
	ErrorThreshold   int           `json:"error_threshold"`
	ResetTimeout    time.Duration `json:"reset_timeout"`
	HalfOpenTimeout time.Duration `json:"half_open_timeout"`
	Enabled         bool          `json:"enabled"`
}

// LoadConfig loads configuration from environment and config file
func LoadConfig(configPath string) (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error loading .env file: %w", err)
	}

	// Read config file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Set defaults for mempool config
	if config.MempoolConfig.MaxPendingTx == 0 {
		config.MempoolConfig.MaxPendingTx = 10000
	}
	if config.MempoolConfig.BlockBufferSize == 0 {
		config.MempoolConfig.BlockBufferSize = 50
	}
	if config.MempoolConfig.GasBoostFactor == 0 {
		config.MempoolConfig.GasBoostFactor = 1.2
	}
	if config.MempoolConfig.DataDir == "" {
		config.MempoolConfig.DataDir = "/tmp/mevbot_mempool.mmap"
	}
	if config.MempoolConfig.Workers == 0 {
		config.MempoolConfig.Workers = 4
	}

	// Set defaults for network config
	if config.Network.ChainID == 0 {
		config.Network.ChainID = 1 // Default to mainnet
	}
	if config.Network.RPCEndpoint == "" {
		config.Network.RPCEndpoint = os.Getenv("ETHEREUM_MAINNET_RPC_URL")
	}
	if config.Network.WSEndpoint == "" {
		config.Network.WSEndpoint = os.Getenv("ETHEREUM_MAINNET_WS_URL")
	}

	// Initialize client
	if config.Client == nil && config.Network.RPCEndpoint != "" {
		client, err := ethclient.Dial(config.Network.RPCEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
		}
		config.Client = client
	}

	// Initialize logger
	if config.Logger == nil {
		logger, err := config.GetLogger()
		if err != nil {
			return nil, fmt.Errorf("failed to create logger: %w", err)
		}
		config.Logger = logger
	}

	// Set defaults for circuit breaker
	if config.CircuitBreaker == nil {
		config.CircuitBreaker = &CircuitBreakerConfig{
			ErrorThreshold: 5,
			ResetTimeout:  time.Minute * 5,
			HalfOpenTimeout: time.Minute,
			Enabled: true,
		}
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// NewConfig creates a new configuration with default values
func NewConfig() (*Config, error) {
	config := &Config{
		Network: struct {
			MainnetRPC     string   `json:"mainnet_rpc"`
			FlashbotsRelay string   `json:"flashbots_relay"`
			WSEndpoints    []string `json:"ws_endpoints"`
			ChainID        int64    `json:"chain_id"`
			RPCEndpoint    string   `json:"rpc_endpoint"`
			WSEndpoint     string   `json:"ws_endpoint"`
		}{
			MainnetRPC:     os.Getenv("ETHEREUM_MAINNET_RPC_URL"),
			FlashbotsRelay: "https://relay.flashbots.net",
			WSEndpoints:    []string{os.Getenv("ETHEREUM_MAINNET_WS_URL")},
			ChainID:        1,
			RPCEndpoint:    os.Getenv("ETHEREUM_MAINNET_RPC_URL"),
			WSEndpoint:     os.Getenv("ETHEREUM_MAINNET_WS_URL"),
		},
		MempoolConfig: struct {
			MaxPendingTx       int     `json:"max_pending_tx"`
			BlockBufferSize    int     `json:"block_buffer_size"`
			MinProfitThreshold float64 `json:"min_profit_threshold"`
			GasBoostFactor    float64 `json:"gas_boost_factor"`
			DataDir           string  `json:"data_dir"`
			Workers           int     `json:"workers"`
		}{
			MaxPendingTx:       10000,
			BlockBufferSize:    50,
			MinProfitThreshold: 0.01,
			GasBoostFactor:     1.2,
			DataDir:            "/tmp/mevbot_mempool.mmap",
			Workers:            4,
		},
		FlashLoan: struct {
			Providers       []string `json:"providers"`
			MaxLoanAmount  float64  `json:"max_loan_amount"`
			MinProfitRatio float64  `json:"min_profit_ratio"`
			MaxFlashLoanFee *big.Int `json:"max_flash_loan_fee"`
		}{
			Providers:      []string{"aave", "dydx", "balancer"},
			MaxLoanAmount:  1000,
			MinProfitRatio: 1.02,
			MaxFlashLoanFee: big.NewInt(1e18), // 1 ETH
		},
		Monitoring: struct {
			PrometheusPort  int    `json:"prometheus_port"`
			HealthCheckPort int    `json:"health_check_port"`
			LogLevel        string `json:"log_level"`
		}{
			PrometheusPort:  9090,
			HealthCheckPort: 8080,
			LogLevel:        "info",
		},
		Security: struct {
			MaxSlippage             float64 `json:"max_slippage"`
			CircuitBreakerThreshold float64 `json:"circuit_breaker_threshold"`
			RateLimitPerSecond      int     `json:"rate_limit_per_second"`
		}{
			MaxSlippage:             0.5,
			CircuitBreakerThreshold: 10,
			RateLimitPerSecond:      100,
		},
		RateLimit: 100.0,
		RateBurst: 10,
		CircuitBreaker: &CircuitBreakerConfig{
			ErrorThreshold:   5,
			ResetTimeout:    time.Minute * 5,
			HalfOpenTimeout: time.Minute,
			Enabled:         true,
		},
		MaxGasPrice: big.NewInt(100e9), // 100 Gwei
	}

	// Initialize client if RPC endpoint is available
	if config.Network.RPCEndpoint != "" {
		client, err := ethclient.Dial(config.Network.RPCEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
		}
		config.Client = client
	}

	// Initialize logger
	logger, err := config.GetLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}
	config.Logger = logger

	return config, nil
}

// validate checks if the configuration is valid
func (c *Config) validate() error {
	if c.Network.RPCEndpoint == "" {
		return fmt.Errorf("RPC endpoint is required")
	}

	if c.Network.ChainID == 0 {
		return fmt.Errorf("chain ID is required")
	}

	if c.MempoolConfig.MaxPendingTx <= 0 {
		return fmt.Errorf("max pending transactions must be positive")
	}

	if c.MempoolConfig.BlockBufferSize <= 0 {
		return fmt.Errorf("block buffer size must be positive")
	}

	if c.MempoolConfig.GasBoostFactor <= 1.0 {
		return fmt.Errorf("gas boost factor must be greater than 1.0")
	}

	if c.FlashLoan.MaxLoanAmount <= 0 {
		return fmt.Errorf("max loan amount must be positive")
	}

	if c.FlashLoan.MinProfitRatio <= 0 {
		return fmt.Errorf("min profit ratio must be positive")
	}

	if c.Security.MaxSlippage <= 0 || c.Security.MaxSlippage >= 100 {
		return fmt.Errorf("max slippage must be between 0 and 100")
	}

	if c.Security.CircuitBreakerThreshold <= 0 {
		return fmt.Errorf("circuit breaker threshold must be positive")
	}

	if c.RouterAddress == "" {
		return fmt.Errorf("router address is required")
	}

	if c.SandwichExecutor == "" {
		return fmt.Errorf("sandwich executor address is required")
	}

	if c.MaxGasPrice == nil {
		return fmt.Errorf("max gas price is required")
	}

	return nil
}

// GetLogger creates a new logger with the configured level
func (c *Config) GetLogger() (*zap.Logger, error) {
	var cfg zap.Config
	if c.Monitoring.LogLevel == "debug" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	if c.Monitoring.LogLevel != "" {
		level, err := zap.ParseAtomicLevel(c.Monitoring.LogLevel)
		if err != nil {
			return nil, fmt.Errorf("invalid log level: %w", err)
		}
		cfg.Level = level
	}

	return cfg.Build()
}
