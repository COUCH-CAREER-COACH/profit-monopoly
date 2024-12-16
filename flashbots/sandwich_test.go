package flashbots

import (
	"context"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

const (
	uniswapV3RouterABI = `[{"inputs":[{"components":[{"internalType":"address","name":"tokenIn","type":"address"},{"internalType":"address","name":"tokenOut","type":"address"},{"internalType":"uint24","name":"fee","type":"uint24"},{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"},{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMinimum","type":"uint256"},{"internalType":"uint160","name":"sqrtPriceLimitX96","type":"uint160"}],"internalType":"struct ISwapRouter.ExactInputSingleParams","name":"params","type":"tuple"}],"name":"exactInputSingle","outputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"}],"stateMutability":"payable","type":"function"}]`
	
	erc20ABI = `[{"constant":false,"inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[{"name":"account","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}]`
	
	wethABI = `[{"constant":false,"inputs":[],"name":"deposit","outputs":[],"payable":true,"stateMutability":"payable","type":"function"}]`
)

func TestSandwichAttack(t *testing.T) {
	// Load environment variables
	err := godotenv.Load("../.env")
	require.NoError(t, err, "Failed to load .env file")

	// Connect to Ethereum node
	client, err := ethclient.Dial(os.Getenv("ETHEREUM_SEPOLIA_RPC_URL"))
	require.NoError(t, err, "Failed to connect to Ethereum node")

	// Get the latest block
	latestBlock, err := client.BlockNumber(context.Background())
	require.NoError(t, err, "Failed to get latest block")

	// Parse private keys
	bundleKey, err := crypto.HexToECDSA(os.Getenv("TESTNET_PRIVATE_KEY"))
	require.NoError(t, err, "Failed to parse bundle key")

	authKey, err := crypto.HexToECDSA(os.Getenv("FLASHBOTS_SIGNER_KEY"))
	require.NoError(t, err, "Failed to parse auth key")

	// Create Flashbots client
	flashbotsClient := NewClient(
		os.Getenv("FLASHBOTS_RELAY_URL"),
		authKey,
		bundleKey,
		big.NewInt(11155111), // Sepolia chain ID
	)

	// Get our address
	ourAddress := crypto.PubkeyToAddress(bundleKey.PublicKey)
	t.Logf("Our address: %s", ourAddress.Hex())

	// Parse ABIs
	routerABI, err := abi.JSON(strings.NewReader(uniswapV3RouterABI))
	require.NoError(t, err, "Failed to parse router ABI")

	tokenABI, err := abi.JSON(strings.NewReader(erc20ABI))
	require.NoError(t, err, "Failed to parse ERC20 ABI")

	wethABI, err := abi.JSON(strings.NewReader(wethABI))
	require.NoError(t, err, "Failed to parse WETH ABI")

	// Addresses (Sepolia)
	weth := common.HexToAddress("0x7b79995e5f793A07Bc00c21412e50Ecae098E7f9")  // Sepolia WETH
	usdc := common.HexToAddress("0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238")  // Sepolia USDC
	uniswapRouter := common.HexToAddress("0x3bFA4769FB09eefC5a80d6E87c3B9C650f7Ae48E")  // Uniswap V3 Router

	// For testing, we'll use a hardcoded amount
	frontRunAmount := new(big.Int).Mul(big.NewInt(1e16), big.NewInt(1)) // 0.01 ETH
	t.Logf("Front-run amount: %s WETH", frontRunAmount.String())

	// Create WETH deposit transaction
	depositTx, err := createWETHDepositTx(wethABI, weth, frontRunAmount)
	require.NoError(t, err, "Failed to create WETH deposit tx")

	// Create approval transactions
	approvalTx, err := createApprovalTx(tokenABI, weth, uniswapRouter, frontRunAmount, ourAddress)
	require.NoError(t, err, "Failed to create approval tx")

	// Create sandwich transactions
	frontRunTx, err := createSwapV3Tx(routerABI, uniswapRouter, weth, usdc, frontRunAmount, ourAddress)
	require.NoError(t, err, "Failed to create front-run tx")

	backRunTx, err := createSwapV3Tx(routerABI, uniswapRouter, usdc, weth, calculateBackRunAmount(frontRunAmount), ourAddress)
	require.NoError(t, err, "Failed to create back-run tx")

	// Sign transactions
	targetBlock := big.NewInt(int64(latestBlock + 1))
	signedDeposit, err := types.SignTx(depositTx, types.NewEIP155Signer(big.NewInt(11155111)), bundleKey)
	require.NoError(t, err, "Failed to sign deposit tx")

	signedApproval, err := types.SignTx(approvalTx, types.NewEIP155Signer(big.NewInt(11155111)), bundleKey)
	require.NoError(t, err, "Failed to sign approval tx")

	signedFrontRun, err := types.SignTx(frontRunTx, types.NewEIP155Signer(big.NewInt(11155111)), bundleKey)
	require.NoError(t, err, "Failed to sign front-run tx")

	signedBackRun, err := types.SignTx(backRunTx, types.NewEIP155Signer(big.NewInt(11155111)), bundleKey)
	require.NoError(t, err, "Failed to sign back-run tx")

	// Marshal transactions
	depositBytes, err := signedDeposit.MarshalBinary()
	require.NoError(t, err, "Failed to marshal deposit tx")

	approvalBytes, err := signedApproval.MarshalBinary()
	require.NoError(t, err, "Failed to marshal approval tx")

	frontRunBytes, err := signedFrontRun.MarshalBinary()
	require.NoError(t, err, "Failed to marshal front-run tx")
	
	backRunBytes, err := signedBackRun.MarshalBinary()
	require.NoError(t, err, "Failed to marshal back-run tx")

	// Create bundle
	bundle := &Bundle{
		Txs: [][]byte{
			depositBytes,
			approvalBytes,
			frontRunBytes,
			backRunBytes,
		},
		BlockNumber: targetBlock,
		MinTimestamp: big.NewInt(time.Now().Unix()),
		MaxTimestamp: big.NewInt(time.Now().Add(time.Minute).Unix()),
	}

	// Simulate bundle
	simulation, err := flashbotsClient.SimulateBundle(bundle)
	require.NoError(t, err, "Failed to simulate bundle")

	// Log simulation results
	t.Logf("Bundle Simulation Results:")
	t.Logf("Success: %v", simulation.Success)
	t.Logf("Error: %s", simulation.Error)
	t.Logf("Gas Used: %d", simulation.GasUsed)
	t.Logf("ETH Sent: %s", simulation.EthSent)
	t.Logf("ETH Received: %s", simulation.EthReceived)
	t.Logf("Profit: %s wei", simulation.ProfitInWei)

	// Only send bundle if simulation was successful and profitable
	if simulation.Success && simulation.ProfitInWei.Cmp(big.NewInt(0)) > 0 {
		err = flashbotsClient.SendBundle(bundle)
		require.NoError(t, err, "Failed to send bundle")
		t.Log("Bundle sent successfully")
	} else {
		t.Log("Bundle not sent: simulation unsuccessful or unprofitable")
	}
}

func calculateBackRunAmount(frontRunAmount *big.Int) *big.Int {
	// Simple calculation: expected USDC output + 1%
	// In practice, this would be calculated based on price impact
	return new(big.Int).Mul(frontRunAmount, big.NewInt(101))
}

func createWETHDepositTx(parsedABI abi.ABI, weth common.Address, amount *big.Int) (*types.Transaction, error) {
	data, err := parsedABI.Pack("deposit")
	if err != nil {
		return nil, err
	}

	return types.NewTransaction(
		0, // nonce will be set by the node
		weth,
		amount,
		100000, // gas limit
		big.NewInt(2e9), // 2 gwei
		data,
	), nil
}

func createApprovalTx(parsedABI abi.ABI, token, spender common.Address, amount *big.Int, from common.Address) (*types.Transaction, error) {
	data, err := parsedABI.Pack("approve", spender, amount)
	if err != nil {
		return nil, err
	}

	return types.NewTransaction(
		0, // nonce will be set by the node
		token,
		big.NewInt(0),
		100000, // gas limit
		big.NewInt(2e9), // 2 gwei
		data,
	), nil
}

func createSwapV3Tx(parsedABI abi.ABI, router, tokenIn, tokenOut common.Address, amountIn *big.Int, recipient common.Address) (*types.Transaction, error) {
	params := struct {
		TokenIn           common.Address
		TokenOut          common.Address
		Fee               *big.Int
		Recipient         common.Address
		Deadline          *big.Int
		AmountIn          *big.Int
		AmountOutMinimum  *big.Int
		SqrtPriceLimitX96 *big.Int
	}{
		TokenIn:           tokenIn,
		TokenOut:          tokenOut,
		Fee:               big.NewInt(3000), // 0.3%
		Recipient:         recipient,
		Deadline:          big.NewInt(time.Now().Add(time.Minute).Unix()),
		AmountIn:          amountIn,
		AmountOutMinimum:  big.NewInt(0), // No minimum output (for testing only)
		SqrtPriceLimitX96: big.NewInt(0), // No price limit
	}
	
	data, err := parsedABI.Pack("exactInputSingle", params)
	if err != nil {
		return nil, err
	}

	return types.NewTransaction(
		0, // nonce will be set by the node
		router,
		big.NewInt(0),
		500000, // gas limit
		big.NewInt(2e9), // 2 gwei
		data,
	), nil
}
