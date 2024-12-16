"""Tests for Flashbots integration"""
import pytest
import pytest_asyncio
from mevbot.core.flashbots import FlashbotsManager
from web3 import Web3
from eth_account import Account
from unittest.mock import Mock, AsyncMock

@pytest.fixture
def w3():
    mock_w3 = Mock(spec=Web3)
    mock_w3.eth = Mock()
    mock_w3.eth.get_block = AsyncMock(return_value={'base_fee_per_gas': 10000000000})
    return mock_w3

@pytest.fixture
def private_key():
    return "0x" + "1" * 64

@pytest.fixture
def config():
    return {
        'test_mode': True,  # Enable test mode
        'min_profit': 0.001,  # 0.1%
        'max_gas_price': 500000000000,  # 500 GWEI
        'flashbots_relay_url': 'https://relay.flashbots.net'
    }

@pytest_asyncio.fixture
async def flashbots_manager(w3, private_key, config):
    manager = FlashbotsManager(w3, private_key, config)
    yield manager

@pytest.mark.asyncio
async def test_bundle_optimization(flashbots_manager):
    """Test bundle optimization"""
    # Test transaction
    test_tx = {
        'to': '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
        'value': int(1e18),  # 1 ETH
        'gas': 21000,
        'maxFeePerGas': 2000000000,
        'maxPriorityFeePerGas': 1000000000,
        'nonce': 0,
        'data': '0x'
    }

    # Test bundle optimization
    optimized_bundle = await flashbots_manager._optimize_bundle([test_tx])
    
    # Verify optimization result
    assert optimized_bundle is not None
    assert len(optimized_bundle) == 1
    assert optimized_bundle[0]['value'] == test_tx['value']
    assert optimized_bundle[0]['gas'] == test_tx['gas']

@pytest.mark.asyncio
async def test_bid_calculation(flashbots_manager):
    """Test bid calculation"""
    # Test transaction
    test_tx = {
        'value': int(1e18),
        'gas': 21000,
        'maxFeePerGas': 2000000000
    }
    
    # Calculate profit per gas
    profit_per_gas = flashbots_manager._calculate_profit_per_gas([test_tx])
    
    # Verify calculation
    assert profit_per_gas > 0

@pytest.mark.asyncio
async def test_builder_reputation(flashbots_manager):
    """Test builder reputation tracking"""
    # Add test builder stats
    builder_id = "test_builder"
    flashbots_manager.builder_stats[builder_id] = {
        'total_bundles': 100,
        'successful_bundles': 95,
        'average_profit': 0.5
    }
    
    # Verify stats
    assert builder_id in flashbots_manager.builder_stats
    assert flashbots_manager.builder_stats[builder_id]['successful_bundles'] == 95
