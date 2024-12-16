"""Tests for JIT Liquidity Strategy"""
import pytest
from unittest.mock import Mock, patch, MagicMock
from web3 import Web3
from eth_account import Account
import asyncio
from decimal import Decimal

from mevbot.strategies.jit_liquidity import JITLiquidityStrategy, JITParams
from mevbot.core.flash_loan import FlashLoanManager, FlashLoanParams, AAVE_V3
from mevbot.core.flashbots import FlashbotsManager
from mevbot.core.memory import MemoryManager

@pytest.fixture
def web3_mock():
    mock = Mock(spec=Web3)
    # Configure eth attribute
    mock.eth = MagicMock()
    mock.eth.contract = MagicMock()
    mock.eth.get_transaction_count = MagicMock(return_value=0)
    mock.eth.gas_price = Web3.to_wei(50, 'gwei')
    mock.eth.get_block = MagicMock(return_value={'baseFeePerGas': 30_000_000_000})
    mock.eth.max_priority_fee = 2_000_000_000
    return mock

@pytest.fixture
def flashbots_mock():
    mock = Mock(spec=FlashbotsManager)
    mock.send_bundle = MagicMock(return_value=True)
    mock.simulate_bundle = MagicMock(return_value=(True, Web3.to_wei(0.1, 'ether')))
    return mock

@pytest.fixture
def memory_mock():
    mock = Mock(spec=MemoryManager)
    mock.allocate_region = MagicMock(return_value=True)
    return mock

@pytest.fixture
def flash_loan_mock():
    mock = Mock(spec=FlashLoanManager)
    mock.get_optimal_provider = MagicMock(return_value=AAVE_V3)
    mock.execute_flash_loan = MagicMock(return_value=(True, {'status': 1}))
    return mock

@pytest.fixture
def strategy(web3_mock, flashbots_mock, memory_mock, flash_loan_mock):
    config = {
        'address': '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
        'private_key': '0x123...',  # Test private key
        'min_profit': 0.001,  # 0.001 ETH
        'max_gas_price': 100e9,  # 100 GWEI
        'slippage_tolerance': 0.005,  # 0.5%
    }
    return JITLiquidityStrategy(web3_mock, flashbots_mock, memory_mock, flash_loan_mock, config)

@pytest.mark.asyncio
async def test_analyze_opportunity_profitable(strategy):
    # Mock data
    target_tx = {
        'hash': '0x123...',
        'value': Web3.to_wei(5, 'ether'),
        'gasPrice': Web3.to_wei(50, 'gwei')
    }
    
    pool_info = {
        'address': '0x456...',
        'token': '0x789...',
        'tvl': Web3.to_wei(1000, 'ether')
    }
    
    # Mock calculations
    strategy.simd.calculate_price_impacts = Mock(return_value=[0.01, 0.02, 0.03, 0.04])
    strategy._get_reserves = Mock(return_value=(1000e18, 1000e18))
    
    # Test
    result = await strategy.analyze_opportunity(target_tx, pool_info)
    
    assert result is not None
    assert result.pool_address == pool_info['address']
    assert result.token_address == pool_info['token']
    assert result.expected_profit > 0

@pytest.mark.asyncio
async def test_analyze_opportunity_unprofitable(strategy):
    # Mock data with low value transaction
    target_tx = {
        'hash': '0x123...',
        'value': Web3.to_wei(0.1, 'ether'),  # Too small
        'gasPrice': Web3.to_wei(150, 'gwei')  # Too high gas
    }
    
    pool_info = {
        'address': '0x456...',
        'token': '0x789...',
        'tvl': Web3.to_wei(10, 'ether')  # Too low TVL
    }
    
    # Test
    result = await strategy.analyze_opportunity(target_tx, pool_info)
    assert result is None

@pytest.mark.asyncio
async def test_execute_opportunity_success(strategy):
    # Mock data
    params = JITParams(
        pool_address='0x456...',
        token_address='0x789...',
        target_tx_hash='0x123...',
        liquidity_amount=Web3.to_wei(1, 'ether'),
        expected_profit=Web3.to_wei(0.01, 'ether')
    )
    
    # Mock flash loan success
    strategy.flash_loan.get_optimal_provider.return_value = AAVE_V3
    strategy.flash_loan.execute_flash_loan.return_value = (True, {'status': 1})
    
    # Test
    success = await strategy.execute_opportunity(params)
    
    assert success is True
    assert strategy.successful_attempts == 1
    assert strategy.total_attempts == 1
    assert strategy.total_profit > 0

@pytest.mark.asyncio
async def test_execute_opportunity_failure(strategy):
    # Mock data
    params = JITParams(
        pool_address='0x456...',
        token_address='0x789...',
        target_tx_hash='0x123...',
        liquidity_amount=Web3.to_wei(1, 'ether'),
        expected_profit=Web3.to_wei(0.01, 'ether')
    )
    
    # Mock flash loan failure
    strategy.flash_loan.get_optimal_provider.return_value = None
    
    # Test
    success = await strategy.execute_opportunity(params)
    
    assert success is False
    assert strategy.successful_attempts == 0
    assert strategy.total_attempts == 1

@pytest.mark.asyncio
async def test_gas_price_limits(strategy):
    # Mock high gas price
    strategy.w3.eth.gas_price = Web3.to_wei(200, 'gwei')  # Above limit
    
    params = JITParams(
        pool_address='0x456...',
        token_address='0x789...',
        target_tx_hash='0x123...',
        liquidity_amount=Web3.to_wei(1, 'ether'),
        expected_profit=Web3.to_wei(0.01, 'ether')
    )
    
    # Should not execute due to high gas
    success = await strategy.execute_opportunity(params)
    assert success is False

@pytest.mark.asyncio
async def test_slippage_protection(strategy):
    # Mock high slippage scenario
    strategy.simd.calculate_price_impacts = Mock(return_value=[0.06, 0.07, 0.08, 0.09])  # Above tolerance
    
    target_tx = {
        'hash': '0x123...',
        'value': Web3.to_wei(5, 'ether'),
        'gasPrice': Web3.to_wei(50, 'gwei')
    }
    
    pool_info = {
        'address': '0x456...',
        'token': '0x789...',
        'tvl': Web3.to_wei(1000, 'ether')
    }
    
    # Should reject opportunity due to high slippage
    result = await strategy.analyze_opportunity(target_tx, pool_info)
    assert result is None
