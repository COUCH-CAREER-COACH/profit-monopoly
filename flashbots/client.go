package flashbots

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	contentTypeJSON    = "application/json"
	flashbotsXHeader   = "X-Flashbots-Signature"
	methodGetUserStats = "flashbots_getUserStats"
	methodSendBundle   = "eth_sendBundle"
)

// Client represents a Flashbots RPC client
type Client struct {
	httpClient  *http.Client
	relayURL    string
	authSigner  *ecdsa.PrivateKey
	chainID     *big.Int
	bundleSigner *ecdsa.PrivateKey
}

// NewClient creates a new Flashbots client
func NewClient(relayURL string, authKey *ecdsa.PrivateKey, bundleKey *ecdsa.PrivateKey, chainID *big.Int) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: time.Second * 3,
		},
		relayURL:     relayURL,
		authSigner:   authKey,
		bundleSigner: bundleKey,
		chainID:      chainID,
	}
}

// Bundle represents a Flashbots transaction bundle
type Bundle struct {
	Txs           [][]byte   // RLP-encoded transactions
	BlockNumber   *big.Int   // Target block number
	MinTimestamp  *big.Int   // Optional: Minimum timestamp for the bundle
	MaxTimestamp  *big.Int   // Optional: Maximum timestamp for the bundle
	RevertingTxHashes []common.Hash // Optional: Tx hashes allowed to revert
}

// BundleSimulation represents the result of simulating a bundle
type BundleSimulation struct {
	Success        bool
	Error          string
	GasUsed        uint64
	EthSent        *big.Int
	EthReceived    *big.Int
	ProfitInWei    *big.Int
	StateBlockNumber uint64
}

// BundleRequest represents a bundle request to Flashbots
type BundleRequest struct {
	Version    string          `json:"version"`
	BlockNum   string          `json:"block"`
	Txs        []string        `json:"transactions"`
	StateBlock string          `json:"stateBlockNumber"`
	Timestamp  uint64          `json:"timestamp,omitempty"`
}

// SendBundle sends a bundle to Flashbots
func (c *Client) SendBundle(bundle *Bundle) error {
	params := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  methodSendBundle,
		"params": []interface{}{
			map[string]interface{}{
				"txs":          bundle.Txs,
				"blockNumber":  fmt.Sprintf("0x%x", bundle.BlockNumber),
				"minTimestamp": bundle.MinTimestamp,
				"maxTimestamp": bundle.MaxTimestamp,
				"revertingTxHashes": bundle.RevertingTxHashes,
			},
		},
	}

	payload, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal bundle: %w", err)
	}

	req, err := http.NewRequest("POST", c.relayURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	signature, err := crypto.Sign(
		accounts.TextHash([]byte(hexutil.Encode(crypto.Keccak256(payload)))),
		c.authSigner,
	)
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	header := fmt.Sprintf("%s:%s",
		crypto.PubkeyToAddress(c.authSigner.PublicKey).Hex(),
		hexutil.Encode(signature),
	)

	req.Header.Add("Content-Type", contentTypeJSON)
	req.Header.Add("Accept", contentTypeJSON)
	req.Header.Add(flashbotsXHeader, header)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("flashbots request failed: %s", string(body))
	}

	return nil
}

// SimulateBundle simulates a bundle before sending
func (c *Client) SimulateBundle(bundle *Bundle) (*BundleSimulation, error) {
	params := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_callBundle",
		"params": []interface{}{
			map[string]interface{}{
				"txs":              bundle.Txs,
				"blockNumber":      fmt.Sprintf("0x%x", bundle.BlockNumber),
				"stateBlockNumber": fmt.Sprintf("0x%x", bundle.BlockNumber.Uint64()-1),
				"timestamp":        time.Now().Unix(),
			},
		},
	}

	payload, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal simulation params: %w", err)
	}

	req, err := http.NewRequest("POST", c.relayURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create simulation request: %w", err)
	}

	signature, err := crypto.Sign(
		accounts.TextHash([]byte(hexutil.Encode(crypto.Keccak256(payload)))),
		c.authSigner,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign simulation request: %w", err)
	}

	header := fmt.Sprintf("%s:%s",
		crypto.PubkeyToAddress(c.authSigner.PublicKey).Hex(),
		hexutil.Encode(signature),
	)

	req.Header.Add("Content-Type", contentTypeJSON)
	req.Header.Add("Accept", contentTypeJSON)
	req.Header.Add(flashbotsXHeader, header)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send simulation request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			Success        bool   `json:"success"`
			Error         string `json:"error"`
			GasUsed       string `json:"gasUsed"`
			EthSent       string `json:"ethSent"`
			EthReceived   string `json:"ethReceived"`
			StateBlock    string `json:"stateBlock"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode simulation response: %w", err)
	}

	gasUsed, _ := hexutil.DecodeUint64(result.Result.GasUsed)
	ethSent, _ := hexutil.DecodeBig(result.Result.EthSent)
	ethReceived, _ := hexutil.DecodeBig(result.Result.EthReceived)
	stateBlock, _ := hexutil.DecodeUint64(result.Result.StateBlock)

	var profit *big.Int
	if ethReceived != nil && ethSent != nil {
		profit = new(big.Int).Sub(ethReceived, ethSent)
	}

	return &BundleSimulation{
		Success:          result.Result.Success,
		Error:           result.Result.Error,
		GasUsed:         gasUsed,
		EthSent:         ethSent,
		EthReceived:     ethReceived,
		ProfitInWei:     profit,
		StateBlockNumber: stateBlock,
	}, nil
}

// GetStats retrieves user stats from Flashbots
func (c *Client) GetStats(blockNumber *big.Int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  methodGetUserStats,
		"params": []interface{}{
			fmt.Sprintf("0x%x", blockNumber),
		},
	}

	payload, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req, err := http.NewRequest("POST", c.relayURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	signature, err := crypto.Sign(
		accounts.TextHash([]byte(hexutil.Encode(crypto.Keccak256(payload)))),
		c.authSigner,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	header := fmt.Sprintf("%s:%s",
		crypto.PubkeyToAddress(c.authSigner.PublicKey).Hex(),
		hexutil.Encode(signature),
	)

	req.Header.Add("Content-Type", contentTypeJSON)
	req.Header.Add("Accept", contentTypeJSON)
	req.Header.Add(flashbotsXHeader, header)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}
