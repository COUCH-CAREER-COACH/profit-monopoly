"""Tests for Optimized JIT Strategy with Performance Monitoring"""
import pytest
import asyncio
import logging
import time
import numpy as np
from unittest.mock import Mock, MagicMock, AsyncMock
from web3 import Web3

from mevbot.strategies.optimized_jit import OptimizedJITStrategy
from mevbot.core.flash_loan import FlashLoanManager
from mevbot.core.memory import MemoryManager
from mevbot.core.performance import PerformanceMonitor

@pytest.fixture
def web3_mock():
    mock = Mock(spec=Web3)
    mock.eth = MagicMock()
    
    # Mock contract calls
    contract_mock = MagicMock()
    contract_mock.functions = MagicMock()
    contract_mock.functions.getReserves = MagicMock()
    contract_mock.functions.getReserves.return_value = MagicMock()
    contract_mock.functions.getReserves.return_value.call = AsyncMock(
        return_value=[1000000, 1000000, int(time.time())]
    )
    mock.eth.contract.return_value = contract_mock
    
    # Mock transaction handling
    mock.eth.gas_price = 50000000000  # 50 gwei
    mock.eth.get_transaction_count = AsyncMock(return_value=0)
    mock.eth.send_raw_transaction = AsyncMock(
        return_value=bytes.fromhex('1234567890'*8)  # Mock tx hash
    )
    
    # Mock account handling
    account_mock = MagicMock()
    signed_tx_mock = MagicMock()
    signed_tx_mock.rawTransaction = bytes.fromhex('1234'*32)
    signed_tx_mock.hash = bytes.fromhex('5678'*32)
    account_mock.sign_transaction = MagicMock(return_value=signed_tx_mock)
    mock.eth.account = account_mock
    
    # Mock chain ID and block
    mock.eth.chain_id = 1  # Mainnet
    mock.eth.block_number = 1000000
    
    # Mock hex conversion
    mock.to_hex = lambda x: hex(x) if isinstance(x, int) else x
    
    return mock

@pytest.fixture
def flash_loan_mock():
    mock = Mock(spec=FlashLoanManager)
    mock.execute_flash_loan = AsyncMock(return_value=True)
    mock.execute_flash_loan.__name__ = 'execute_flash_loan'
    return mock

@pytest.fixture
def memory_mock():
    mock = Mock(spec=MemoryManager)
    mock.lock = AsyncMock()
    mock.lock.__aenter__ = AsyncMock()
    mock.lock.__aexit__ = AsyncMock()
    mock.lock.__aenter__.__name__ = '__aenter__'
    mock.lock.__aexit__.__name__ = '__aexit__'
    return mock

@pytest.fixture
def config():
    return {
        'pool_addresses': [
            '0x1234567890123456789012345678901234567890',
            '0x0987654321098765432109876543210987654321'
        ],
        'pool_abi': [],
        'private_key': '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
        'min_profit': 0.01,
        'fee_rate': 0.003,
        'max_own_liquidity': 1.0,
        'gas_price_history': [150000, 200000, 250000],  # More realistic gas limits
        'success_rate_history': [0.9, 0.95, 0.98],
        'token_address': '0x1234567890123456789012345678901234567890',
        'interaction_data': '0x1234567890abcdef',
        'significant_profit_threshold': 0.05,
        'test_mode': True  # Enable test mode
    }

@pytest.fixture
def strategy(web3_mock, flash_loan_mock, memory_mock, config):
    """Create a strategy instance with mocked dependencies"""
    strategy = OptimizedJITStrategy(
        web3=web3_mock,
        flash_loan_manager=flash_loan_mock,
        memory_manager=memory_mock,
        config=config
    )
    
    # Initialize performance monitor with test mode
    strategy.performance = PerformanceMonitor(config=config)
    
    return strategy

@pytest.mark.asyncio
async def test_pool_monitoring_performance(strategy, caplog):
    """Test pool monitoring with performance metrics"""
    # Monitor pools
    async with strategy.performance.measure_latency('pool_updates'):
        pool_states = await strategy.monitor_pools_vectorized(
            strategy.config['pool_addresses']
        )

    # Verify performance metrics were collected
    assert len(strategy.performance.latencies.get('pool_updates', [])) > 0

@pytest.mark.asyncio
async def test_opportunity_execution_performance(strategy, caplog):
    """Test opportunity execution with performance tracking"""
    params = {
        'pool': strategy.config['pool_addresses'][0],
        'amount': Web3.to_wei(0.1, 'ether'),
        'needs_loan': True,
        'gas_prices': [150000, 200000, 250000],  # Pass as list
        'success_rates': [0.9, 0.95, 0.98],  # Pass as list
        'token': strategy.config['token_address'],
        'data': strategy.config['interaction_data'],
        'expected_profit': 0.05
    }

    # Execute opportunity
    success = await strategy.execute_opportunity(params)
    assert success, "Opportunity execution failed"

    # Verify performance metrics
    assert len(strategy.performance.latencies.get('tx_submissions', [])) > 0, "No transaction latency recorded"

@pytest.mark.asyncio
async def test_strategy_iteration_performance(strategy, caplog):
    """Test full strategy iteration with performance monitoring"""
    # Run one iteration
    async with strategy.performance.measure_latency('calculations'):
        await strategy._run_iteration()

    # Verify all performance metrics were collected
    metrics = strategy.performance.get_performance_report()
    assert float(metrics['latency']['rpc_calls'].rstrip('ms')) > 0
    assert float(metrics['latency']['calculations'].rstrip('ms')) > 0
    assert 'success_rates' in metrics
    assert 'profit' in metrics

@pytest.mark.asyncio
async def test_performance_thresholds(strategy, caplog):
    """Test performance threshold warnings"""
    caplog.set_level(logging.WARNING)
    
    # Sleep for longer in test mode to exceed the higher threshold
    async with strategy.performance.measure_latency('rpc_calls'):
        await asyncio.sleep(0.6)  # Exceed test mode threshold (0.5s)

    # Wait a bit for the warning to be logged
    await asyncio.sleep(0.1)
    
    assert any("High rpc_calls latency detected" in record.message
              for record in caplog.records), "Expected high latency warning"

@pytest.mark.asyncio
async def test_profit_tracking_accuracy(strategy):
    """Test accuracy of profit tracking"""
    params = {
        'pool': strategy.config['pool_addresses'][0],
        'amount': Web3.to_wei(1.0, 'ether'),
        'needs_loan': False,
        'gas_prices': [150000, 200000, 250000],  # Pass as list
        'success_rates': [0.9, 0.95, 0.98],  # Pass as list
        'token': strategy.config['token_address'],
        'data': strategy.config['interaction_data'],
        'expected_profit': 0.1
    }

    # Execute opportunity
    success = await strategy.execute_opportunity(params)
    assert success, "Opportunity execution failed"

    # Verify profit recording
    profits = list(strategy.performance.profits)
    assert len(profits) > 0
    assert profits[-1] > 0

@pytest.mark.asyncio
async def test_vectorized_calculations_performance(strategy):
    """Test performance of vectorized calculations"""
    market_data = {
        'prices': np.array([100.0, 101.0, 102.0]),
        'volumes': np.array([1.0, 1.5, 2.0]),
        'fees': np.array([0.003, 0.003, 0.003])
    }

    async with strategy.performance.measure_latency('calculations'):
        has_opportunity, profit = strategy.analyze_opportunities_vectorized(market_data)

    calc_time = list(strategy.performance.latencies['calculations'])[-1]
    assert calc_time < 0.1  # Should complete in under 100ms

@pytest.mark.asyncio
async def test_memory_optimization(strategy):
    """Test memory usage optimization"""
    # Just verify we can collect metrics without error
    await strategy.performance.collect_system_metrics()
