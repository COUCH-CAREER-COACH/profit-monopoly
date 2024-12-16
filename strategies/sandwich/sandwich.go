package sandwich

import (
	"context"
	"fmt"
	"math/big"
	"github.com/michaelpento.lv/mevbot/config"
	"github.com/michaelpento.lv/mevbot/utils"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"go.uber.org/zap"
)

type SandwichAttack struct {
	config     *config.Config
	calculator *utils.ProfitCalculator
	flashloan  *utils.FlashLoan
	bundler    *utils.Bundler
	logger     *zap.Logger
}

func NewSandwichAttack(cfg *config.Config) (*SandwichAttack, error) {
	calculator := utils.NewProfitCalculator(cfg)
	flashloan := utils.NewFlashLoan(cfg)
	bundler, err := utils.NewBundler(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create bundler: %w", err)
	}

	return &SandwichAttack{
		config:     cfg,
		calculator: calculator,
		flashloan:  flashloan,
		bundler:    bundler,
		logger:     cfg.Logger,
	}, nil
}

func (s *SandwichAttack) IsProfitable(tx *types.Transaction) bool {
	// Extract swap parameters from transaction
	swap, err := utils.DecodeSwapParams(tx.Data(), s.logger)
	if err != nil {
		s.logger.Error("Failed to decode swap params", zap.Error(err))
		return false
	}

	// Get target pool
	pool := s.getTargetPool(swap.TokenIn, swap.TokenOut)
	if err := s.validatePool(pool); err != nil {
		s.logger.Error("Invalid pool", zap.Error(err))
		return false
	}

	// Calculate optimal amounts
	frontrunAmount, err := s.calculator.CalculateOptimalFrontrunAmount(pool, swap.AmountIn)
	if err != nil {
		s.logger.Error("Failed to calculate optimal frontrun amount", zap.Error(err))
		return false
	}

	// Calculate expected profit
	profit, err := s.calculator.CalculateExpectedProfit(pool, frontrunAmount, swap.AmountIn)
	if err != nil {
		s.logger.Error("Failed to calculate profit", zap.Error(err))
		return false
	}

	// Check if profit exceeds minimum threshold
	return profit.Cmp(s.config.MinProfitThreshold) >= 0
}

func (s *SandwichAttack) Execute(ctx context.Context, victimTx *types.Transaction) error {
	// Extract swap parameters
	swap, err := utils.DecodeSwapParams(victimTx.Data(), s.logger)
	if err != nil {
		return fmt.Errorf("failed to decode swap params: %w", err)
	}

	// Get target pool
	pool := s.getTargetPool(swap.TokenIn, swap.TokenOut)
	if err := s.validatePool(pool); err != nil {
		return err
	}

	// Calculate optimal amounts
	frontrunAmount, err := s.calculator.CalculateOptimalFrontrunAmount(pool, swap.AmountIn)
	if err != nil {
		return fmt.Errorf("failed to calculate optimal frontrun amount: %w", err)
	}

	// Get flash loan
	flParams := &utils.FlashLoanParams{
		Token:     swap.TokenIn,
		Amount:    frontrunAmount,
		MaxFee:    s.config.MaxFlashLoanFee,
		Recipient: s.config.SandwichExecutor,
	}

	flashLoanTx, err := s.flashloan.Execute(ctx, flParams)
	if err != nil {
		return fmt.Errorf("failed to execute flash loan: %w", err)
	}

	// Create frontrun transaction
	frontrunTx, err := s.createFrontrunTx(frontrunAmount, swap)
	if err != nil {
		return fmt.Errorf("failed to create frontrun tx: %w", err)
	}

	// Create backrun transaction
	backrunTx, err := s.createBackrunTx(frontrunAmount, swap)
	if err != nil {
		return fmt.Errorf("failed to create backrun tx: %w", err)
	}

	// Submit bundle
	bundle := []*types.Transaction{flashLoanTx, frontrunTx, victimTx, backrunTx}
	if err := s.bundler.SubmitBundle(ctx, bundle); err != nil {
		return fmt.Errorf("failed to submit bundle: %w", err)
	}

	return nil
}

func (s *SandwichAttack) getTargetPool(token0, token1 common.Address) *config.PoolConfig {
	for _, pool := range s.config.Pools {
		if (pool.Token0 == token0 && pool.Token1 == token1) ||
			(pool.Token0 == token1 && pool.Token1 == token0) {
			return &pool
		}
	}
	return nil
}

func (s *SandwichAttack) createFrontrunTx(amount *big.Int, swap *utils.SwapParams) (*types.Transaction, error) {
	// Create swap transaction with calculated amount
	decoder, err := utils.NewTransactionDecoder(s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	params := &utils.SwapParams{
		TokenIn:      swap.TokenIn,
		TokenOut:     swap.TokenOut,
		AmountIn:     amount,
		AmountOutMin: big.NewInt(0), // Accept any amount of output tokens
		Path:         []common.Address{swap.TokenIn, swap.TokenOut},
		To:           s.config.SandwichExecutor,
		Deadline:     big.NewInt(time.Now().Add(time.Minute).Unix()),
	}

	data, err := decoder.EncodeSwapExactTokensForTokens(params)
	if err != nil {
		return nil, err
	}

	// Build transaction
	nonce, err := s.config.Client.PendingNonceAt(context.Background(), s.config.SandwichExecutor)
	if err != nil {
		return nil, err
	}

	gasPrice := new(big.Int).Set(s.config.MaxGasPrice)
	gasLimit := uint64(300000) // Estimated gas limit for swap

	tx := types.NewTransaction(
		nonce,
		s.config.RouterAddress,
		big.NewInt(0),
		gasLimit,
		gasPrice,
		data,
	)

	return tx, nil
}

func (s *SandwichAttack) createBackrunTx(amount *big.Int, swap *utils.SwapParams) (*types.Transaction, error) {
	// Create swap transaction to close position
	decoder, err := utils.NewTransactionDecoder(s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	params := &utils.SwapParams{
		TokenIn:      swap.TokenOut,
		TokenOut:     swap.TokenIn,
		AmountIn:     amount,
		AmountOutMin: big.NewInt(0), // Accept any amount of output tokens
		Path:         []common.Address{swap.TokenOut, swap.TokenIn},
		To:           s.config.SandwichExecutor,
		Deadline:     big.NewInt(time.Now().Add(time.Minute).Unix()),
	}

	data, err := decoder.EncodeSwapExactTokensForTokens(params)
	if err != nil {
		return nil, err
	}

	// Build transaction
	nonce, err := s.config.Client.PendingNonceAt(context.Background(), s.config.SandwichExecutor)
	if err != nil {
		return nil, err
	}

	gasPrice := new(big.Int).Set(s.config.MaxGasPrice)
	gasLimit := uint64(300000) // Estimated gas limit for swap

	tx := types.NewTransaction(
		nonce+1, // Increment nonce for backrun
		s.config.RouterAddress,
		big.NewInt(0),
		gasLimit,
		gasPrice,
		data,
	)

	return tx, nil
}

func (s *SandwichAttack) validatePool(pool *config.PoolConfig) error {
	if pool == nil {
		return fmt.Errorf("pool is nil")
	}
	if pool.Reserve0.Sign() <= 0 || pool.Reserve1.Sign() <= 0 {
		return fmt.Errorf("invalid pool reserves")
	}
	return nil
}
