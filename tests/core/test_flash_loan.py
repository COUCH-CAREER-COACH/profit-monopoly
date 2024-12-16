"""Tests for Flash Loan Manager"""
import pytest
from unittest.mock import Mock, patch, MagicMock
from web3 import Web3
from eth_account import Account
import asyncio
from decimal import Decimal

from mevbot.core.flash_loan import (
    FlashLoanManager,
    FlashLoanParams,
    AAVE_V3,
    DYDX,
    BALANCER,
    UNISWAP_V3
)

@pytest.fixture
def web3_mock():
    mock = Mock(spec=Web3)
    # Configure eth attribute
    mock.eth = MagicMock()
    mock.eth.contract = MagicMock()
    mock.eth.get_transaction_count = MagicMock(return_value=0)
    mock.eth.get_block = MagicMock(return_value={'baseFeePerGas': 30_000_000_000})
    mock.eth.max_priority_fee = 2_000_000_000
    mock.eth.send_transaction = MagicMock(return_value='0x123...')
    mock.eth.wait_for_transaction_receipt = MagicMock(return_value={'status': 1})
    return mock

@pytest.fixture
def flash_loan_manager(web3_mock):
    config = {
        'address': '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
        'private_key': '0x123...',  # Test private key
        'aave_lending_pool': '0x7d2768dE32b0b80b7a3454c06BdAc94A69DDc7A9',
        'dydx_solo': '0x1E0447b19BB6EcFdAe1e4AE1694b0C3659614e4e',
        'balancer_vault': '0xBA12222222228d8Ba445958a75a0704d566BF2C8',
        'uniswap_factory': '0x1F98431c8aD98523631AE4a59f267346ea31F984',
    }
    return FlashLoanManager(web3_mock, config)

@pytest.mark.asyncio
async def test_get_optimal_provider(flash_loan_manager):
    # Mock token and amount
    token = '0x6B175474E89094C44Da98b954EedeAC495271d0F'  # DAI
    amount = Web3.to_wei(1000, 'ether')
    
    # Mock provider responses
    flash_loan_manager._get_provider_fee = Mock(return_value=0.001)  # 0.1% fee
    flash_loan_manager._get_provider_liquidity = Mock(return_value=Web3.to_wei(10000, 'ether'))
    
    # Test
    provider = await flash_loan_manager.get_optimal_provider(token, amount)
    assert provider in [AAVE_V3, DYDX, BALANCER, UNISWAP_V3]

@pytest.mark.asyncio
async def test_get_optimal_provider_insufficient_liquidity(flash_loan_manager):
    # Mock token and large amount
    token = '0x6B175474E89094C44Da98b954EedeAC495271d0F'
    amount = Web3.to_wei(100000, 'ether')  # Very large amount
    
    # Mock low liquidity
    flash_loan_manager._get_provider_liquidity = Mock(return_value=Web3.to_wei(1000, 'ether'))
    
    # Test
    provider = await flash_loan_manager.get_optimal_provider(token, amount)
    assert provider is None

@pytest.mark.asyncio
async def test_execute_flash_loan_aave(flash_loan_manager):
    # Mock successful Aave flash loan
    params = FlashLoanParams(
        provider=AAVE_V3,
        token_address='0x6B175474E89094C44Da98b954EedeAC495271d0F',
        amount=Web3.to_wei(1000, 'ether'),
        callback_data=b'',
        expected_profit=Web3.to_wei(1, 'ether')
    )
    
    # Mock transaction success
    flash_loan_manager.w3.eth.send_transaction = Mock(return_value='0x123...')
    flash_loan_manager.w3.eth.wait_for_transaction_receipt = Mock(
        return_value={'status': 1}
    )
    
    # Test
    success, receipt = await flash_loan_manager.execute_flash_loan(params)
    assert success is True
    assert receipt['status'] == 1

@pytest.mark.asyncio
async def test_execute_flash_loan_failure(flash_loan_manager):
    # Mock failed flash loan
    params = FlashLoanParams(
        provider=AAVE_V3,
        token_address='0x6B175474E89094C44Da98b954EedeAC495271d0F',
        amount=Web3.to_wei(1000, 'ether'),
        callback_data=b'',
        expected_profit=Web3.to_wei(1, 'ether')
    )
    
    # Mock transaction failure
    flash_loan_manager.w3.eth.send_transaction = Mock(side_effect=Exception("Transaction failed"))
    
    # Test
    success, receipt = await flash_loan_manager.execute_flash_loan(params)
    assert success is False
    assert receipt is None

@pytest.mark.asyncio
async def test_provider_fees(flash_loan_manager):
    token = '0x6B175474E89094C44Da98b954EedeAC495271d0F'
    amount = Web3.to_wei(1000, 'ether')
    
    # Test each provider's fee
    aave_fee = await flash_loan_manager._get_provider_fee(AAVE_V3, token, amount)
    dydx_fee = await flash_loan_manager._get_provider_fee(DYDX, token, amount)
    balancer_fee = await flash_loan_manager._get_provider_fee(BALANCER, token, amount)
    uniswap_fee = await flash_loan_manager._get_provider_fee(UNISWAP_V3, token, amount)
    
    assert aave_fee == amount * 0.0009  # 0.09%
    assert dydx_fee == 0  # No fee
    assert balancer_fee == amount * 0.0001  # 0.01%
    assert uniswap_fee == amount * 0.0005  # 0.05%

@pytest.mark.asyncio
async def test_build_flash_loan_tx(flash_loan_manager):
    params = FlashLoanParams(
        provider=AAVE_V3,
        token_address='0x6B175474E89094C44Da98b954EedeAC495271d0F',
        amount=Web3.to_wei(1000, 'ether'),
        callback_data=b'',
        expected_profit=Web3.to_wei(1, 'ether')
    )
    
    # Mock nonce and gas prices
    flash_loan_manager.w3.eth.get_transaction_count = Mock(return_value=0)
    flash_loan_manager._get_max_fee = Mock(return_value=50_000_000_000)
    flash_loan_manager._get_priority_fee = Mock(return_value=2_000_000_000)
    
    # Test
    tx = await flash_loan_manager._build_flash_loan_tx(params)
    
    assert tx is not None
    assert tx['from'] == flash_loan_manager.config['address']
    assert tx['nonce'] == 0
    assert tx['gas'] == 500000
    assert tx['maxFeePerGas'] == 50_000_000_000
    assert tx['maxPriorityFeePerGas'] == 2_000_000_000

@pytest.mark.asyncio
async def test_gas_price_management(flash_loan_manager):
    # Mock block data
    flash_loan_manager.w3.eth.get_block = Mock(
        return_value={'baseFeePerGas': 30_000_000_000}
    )
    flash_loan_manager.w3.eth.max_priority_fee = 2_000_000_000
    
    # Test max fee calculation
    max_fee = await flash_loan_manager._get_max_fee()
    assert max_fee == 60_000_000_000  # 2x base fee
    
    # Test priority fee
    priority_fee = await flash_loan_manager._get_priority_fee()
    assert priority_fee == 2_000_000_000
