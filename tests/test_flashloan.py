"""Tests for flash loan functionality"""
import pytest
import pytest_asyncio
from mevbot.core.flashloan import FlashLoanManager, FlashLoanParams
from web3 import Web3
from eth_typing import Address
from unittest.mock import Mock, AsyncMock
import numpy as np

@pytest.fixture
def w3():
    mock_w3 = Mock(spec=Web3)
    mock_w3.eth = Mock()
    return mock_w3

@pytest.fixture
def config():
    return {
        'test_mode': True,  # Enable test mode
        'max_loan_amount': int(1e20),  # 100 ETH
        'min_profit': 0.001  # 0.1%
    }

@pytest_asyncio.fixture
async def flash_loan_manager(w3, config):
    manager = FlashLoanManager(w3, config)
    yield manager

@pytest.mark.asyncio
async def test_optimize_flash_loan(flash_loan_manager):
    """Test flash loan optimization"""
    # Test token and amounts
    test_token = Address('0x' + '1' * 40)
    min_amount = 1000000000000000000  # 1 ETH
    max_amount = 10000000000000000000  # 10 ETH
    test_route = [Address('0x' + '2' * 40), Address('0x' + '3' * 40)]

    # Optimize flash loan
    params = await flash_loan_manager.optimize_flash_loan(
        test_token,
        min_amount,
        max_amount,
        test_route
    )

    # Verify optimization result
    assert params is not None
    assert params.token == test_token
    assert min_amount <= params.amount <= max_amount
    assert params.expected_profit > 0
    assert len(params.route) == len(test_route)

@pytest.mark.asyncio
async def test_provider_monitoring(flash_loan_manager):
    """Test provider monitoring"""
    # Start monitoring
    await flash_loan_manager.start_monitoring()
    
    # Verify provider initialization
    assert len(flash_loan_manager.providers) > 0
    test_provider = next(iter(flash_loan_manager.providers.values()))
    assert test_provider.current_liquidity > 0
    assert test_provider.max_loan_amount > 0
    assert test_provider.fee_percentage > 0
    
    # Stop monitoring
    if flash_loan_manager.monitoring_task:
        flash_loan_manager.monitoring_task.cancel()

@pytest.mark.asyncio
async def test_estimate_profits(flash_loan_manager):
    """Test profit estimation"""
    test_token = Address('0x' + '1' * 40)
    test_amount = 1000000000000000000  # 1 ETH
    test_route = [Address('0x' + '2' * 40), Address('0x' + '3' * 40)]
    
    # Get test provider
    provider = next(iter(flash_loan_manager.providers.values()))
    
    # Create loan params
    params = FlashLoanParams(
        token=test_token,
        amount=test_amount,
        provider=provider,
        route=test_route,
        expected_profit=0
    )
    
    # Test profit estimation
    amounts = np.array([test_amount])
    profits = await flash_loan_manager._estimate_profits(amounts, test_route)
    
    # Verify profit calculation
    assert len(profits) == 1
    assert profits[0] >= 0
    assert provider.fee_percentage > 0

@pytest.mark.asyncio
async def test_execute_flash_loan(flash_loan_manager):
    """Test flash loan execution"""
    test_token = Address('0x' + '1' * 40)
    test_amount = 1000000000000000000  # 1 ETH
    test_route = [Address('0x' + '2' * 40), Address('0x' + '3' * 40)]
    
    # Get test provider
    provider = next(iter(flash_loan_manager.providers.values()))
    
    # Create loan params
    params = FlashLoanParams(
        token=test_token,
        amount=test_amount,
        provider=provider,
        route=test_route,
        expected_profit=test_amount * 0.002  # 0.2% expected profit
    )
    
    # Execute flash loan
    tx_hash = await flash_loan_manager.execute_flash_loan(params)
    
    # In test mode, tx_hash will be None since we don't actually send transactions
    assert tx_hash is None

@pytest.mark.asyncio
async def test_prepare_loan(flash_loan_manager):
    """Test loan preparation"""
    test_token = Address('0x' + '1' * 40)
    test_amount = 1000000000000000000  # 1 ETH
    
    # Prepare loan
    params = await flash_loan_manager.prepare_loan(test_amount, test_token)
    
    # Verify loan parameters
    assert params is not None
    assert params.token == test_token
    assert params.amount == test_amount
    assert params.provider is not None
    assert params.expected_profit >= 0
    assert len(params.route) > 0
