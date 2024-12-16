"""Tests for sandwich attack strategy"""
import pytest
from unittest.mock import Mock, AsyncMock
from web3 import Web3
from eth_typing import Address
from mevbot.strategies.sandwich import SandwichStrategy, SandwichParams
from mevbot.core.flashbots import FlashbotsManager

@pytest.fixture
def w3():
    """Web3 fixture"""
    return Mock(spec=Web3)

@pytest.fixture
def flashbots():
    """Flashbots manager fixture"""
    mock = Mock(spec=FlashbotsManager)
    mock.get_gas_price = AsyncMock(return_value=50e9)  # 50 gwei
    mock.send_bundle = AsyncMock(return_value=True)
    return mock

@pytest.fixture
def config():
    """Strategy config fixture"""
    return {
        'min_profit': 0.01,  # 0.01 ETH
        'max_gas': 500000,
        'slippage_tolerance': 0.02  # 2%
    }

@pytest.fixture
def strategy(w3, flashbots, config):
    """Strategy fixture"""
    return SandwichStrategy(w3, flashbots, config)

@pytest.mark.asyncio
async def test_analyze_opportunity(strategy):
    """Test opportunity analysis"""
    # Mock data
    target_tx = {
        'hash': '0x123',
        'value': int(1e18)  # 1 ETH
    }
    pool_info = {
        'token': Address(bytes([1] * 20)),
        'address': Address(bytes([2] * 20))
    }
    
    # Test analysis
    params = await strategy.analyze_opportunity(target_tx, pool_info)
    
    assert isinstance(params, SandwichParams)
    assert params.target_tx_hash == target_tx['hash']
    assert params.token == pool_info['token']
    assert params.pool == pool_info['address']
    assert params.front_amount > 0
    assert params.back_amount > params.front_amount
    assert params.expected_profit > 0

@pytest.mark.asyncio
async def test_execute_sandwich(strategy):
    """Test sandwich execution"""
    # Create test parameters
    params = SandwichParams(
        target_tx_hash='0x123',
        token=Address(bytes([1] * 20)),
        pool=Address(bytes([2] * 20)),
        front_amount=int(0.5e18),  # 0.5 ETH
        back_amount=int(0.51e18),  # 0.51 ETH
        expected_profit=0.02  # 0.02 ETH
    )
    
    # Execute sandwich
    success = await strategy.execute_sandwich(params)
    
    assert success
    assert strategy.total_attempts == 1
    assert strategy.successful_attempts == 1
    assert strategy.total_profit > 0
    assert strategy.total_gas_used > 0

@pytest.mark.asyncio
async def test_failed_sandwich(strategy):
    """Test failed sandwich execution"""
    # Mock failed bundle submission
    strategy.flashbots.send_bundle = AsyncMock(return_value=False)
    
    params = SandwichParams(
        target_tx_hash='0x123',
        token=Address(bytes([1] * 20)),
        pool=Address(bytes([2] * 20)),
        front_amount=int(0.5e18),
        back_amount=int(0.51e18),
        expected_profit=0.02
    )
    
    success = await strategy.execute_sandwich(params)
    
    assert not success
    assert strategy.total_attempts == 1
    assert strategy.successful_attempts == 0
    assert strategy.total_profit == 0

@pytest.mark.asyncio
async def test_invalid_opportunity(strategy):
    """Test invalid opportunity analysis"""
    # Test with zero value transaction
    target_tx = {
        'hash': '0x123',
        'value': 0
    }
    pool_info = {
        'token': Address(bytes([1] * 20)),
        'address': Address(bytes([2] * 20))
    }
    
    params = await strategy.analyze_opportunity(target_tx, pool_info)
    assert params is None
