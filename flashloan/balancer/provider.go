package balancer

import (
	"context"
	"fmt"
	"math/big"
	"github.com/michaelpento.lv/mevbot/flashloan"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

const (
	// Mainnet addresses
	VaultAddress = "0xBA12222222228d8Ba445958a75a0704d566BF2C8"
)

// Provider implements the flashloan.Provider interface for Balancer
type Provider struct {
	client *ethclient.Client
	vault  *bind.BoundContract
	logger *zap.Logger
}

// NewProvider creates a new Balancer flash loan provider
func NewProvider(client *ethclient.Client, logger *zap.Logger) (*Provider, error) {
	vaultAddr := common.HexToAddress(VaultAddress)
	vault, err := bind.NewBoundContract(vaultAddr, vaultABI, client, client, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault contract: %w", err)
	}

	return &Provider{
		client: client,
		vault:  vault,
		logger: logger,
	}, nil
}

// ExecuteFlashLoan executes a flash loan through Balancer
func (p *Provider) ExecuteFlashLoan(ctx context.Context, params flashloan.FlashLoanParams) (*types.Transaction, error) {
	p.logger.Info("Executing Balancer flash loan",
		zap.String("token", params.Token.Hex()),
		zap.String("amount", params.Amount.String()))

	// Pack flash loan data
	data, err := p.packFlashLoanData(params)
	if err != nil {
		return nil, fmt.Errorf("failed to pack flash loan data: %w", err)
	}

	// Get gas price
	gasPrice, err := p.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	// Create transaction
	auth, err := bind.NewKeyedTransactorWithChainID(nil, big.NewInt(1)) // Mainnet
	if err != nil {
		return nil, fmt.Errorf("failed to create auth: %w", err)
	}
	auth.GasPrice = gasPrice
	auth.GasLimit = uint64(1000000) // Set appropriate gas limit

	// Execute flash loan
	tx, err := p.vault.Transact(auth, "flashLoan", params.Token, params.Amount, params.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to execute flash loan: %w", err)
	}

	p.logger.Info("Flash loan executed",
		zap.String("tx_hash", tx.Hash().Hex()),
		zap.String("token", params.Token.Hex()),
		zap.String("amount", params.Amount.String()))

	return tx, nil
}

// GetFlashLoanFee returns the flash loan fee for Balancer (0%)
func (p *Provider) GetFlashLoanFee(ctx context.Context, token common.Address) (*big.Int, error) {
	return big.NewInt(0), nil // Balancer has no flash loan fees
}

// GetLiquidity returns the available liquidity for a token in Balancer
func (p *Provider) GetLiquidity(ctx context.Context, token common.Address) (*big.Int, error) {
	var result struct {
		Cash      *big.Int
		Borrowed  *big.Int
		Reserved  *big.Int
	}

	err := p.vault.Call(&bind.CallOpts{Context: ctx}, &result, "getPoolTokenInfo", token)
	if err != nil {
		return nil, fmt.Errorf("failed to get token info: %w", err)
	}

	// Available liquidity is cash minus reserved
	liquidity := new(big.Int).Sub(result.Cash, result.Reserved)
	if liquidity.Sign() < 0 {
		liquidity = big.NewInt(0)
	}

	return liquidity, nil
}

// String returns the provider name
func (p *Provider) String() string {
	return "Balancer"
}

// packFlashLoanData packs the flash loan parameters into calldata
func (p *Provider) packFlashLoanData(params flashloan.FlashLoanParams) ([]byte, error) {
	// Pack flash loan parameters according to Balancer's expected format
	args := abi.Arguments{
		{Type: abiUint256},  // amount
		{Type: abiAddress},  // token
		{Type: abiBytes},    // userData
	}

	packed, err := args.Pack(
		params.Amount,
		params.Token,
		params.Data,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pack parameters: %w", err)
	}

	return packed, nil
}

// ABI types
var (
	abiUint256, _ = abi.NewType("uint256", "", nil)
	abiAddress, _ = abi.NewType("address", "", nil)
	abiBytes, _   = abi.NewType("bytes", "", nil)
)

// Vault ABI
var vaultABI, _ = abi.JSON(strings.NewReader(`[
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "recipient",
				"type": "address"
			},
			{
				"internalType": "contract IERC20",
				"name": "token",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "amount",
				"type": "uint256"
			},
			{
				"internalType": "bytes",
				"name": "userData",
				"type": "bytes"
			}
		],
		"name": "flashLoan",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "token",
				"type": "address"
			}
		],
		"name": "getPoolTokenInfo",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "cash",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "borrowed",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "reserved",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`))
