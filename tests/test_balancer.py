import pytest
import pytest_asyncio
from web3 import Web3
from eth_typing import Address
from mevbot.core.flashloan import FlashLoanManager, FlashLoanProvider
import os
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

@pytest.fixture
def w3():
    """Create Web3 instance"""
    return Web3(Web3.HTTPProvider('https://eth-mainnet.g.alchemy.com/v2/your-api-key'))

@pytest.fixture
def config():
    """Test configuration"""
    return {
        'test_mode': True,
        'balancer_vault': '0xBA12222222228d8Ba445958a75a0704d566BF2C8',
        'balancer_pool_id': '0x5c6ee304399dbdb9c8ef030ab642b10820db8f56000200000000000000000014',
        'weth_address': '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
        'min_profit': 0.001,
        'max_gas_price': 100e9,
        'balancer_vault_abi': [
            {
                "inputs": [{"internalType": "bytes32", "name": "poolId", "type": "bytes32"}],
                "name": "getPoolTokens",
                "outputs": [
                    {"internalType": "address[]", "name": "tokens", "type": "address[]"},
                    {"internalType": "uint256[]", "name": "balances", "type": "uint256[]"},
                    {"internalType": "uint256", "name": "lastChangeBlock", "type": "uint256"}
                ],
                "stateMutability": "view",
                "type": "function"
            }
        ]
    }

@pytest_asyncio.fixture
async def flash_loan_manager(w3, config):
    """Create FlashLoanManager instance"""
    manager = FlashLoanManager(w3, config)
    await manager.start_monitoring()
    yield manager
    await manager.stop_monitoring()

@pytest.mark.asyncio
async def test_balancer_liquidity_monitoring(flash_loan_manager):
    """Test Balancer liquidity monitoring"""
    provider = flash_loan_manager.providers.get('balancer')
    assert provider is not None
    assert provider.current_liquidity > 0
    assert provider.max_loan_amount > 0
    assert provider.fee_percentage == 0.0001

@pytest.mark.asyncio
async def test_balancer_flash_loan_preparation(flash_loan_manager):
    """Test preparing Balancer flash loan"""
    amount = int(1e18)  # 1 ETH
    token = Address('0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2')  # WETH
    route = [
        Address('0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2'),
        Address('0x6B175474E89094C44Da98b954EedeAC495271d0F')  # DAI
    ]
    
    params = await flash_loan_manager.prepare_loan(amount, token, route)
    assert params is not None
    assert params.amount == amount
    assert params.token == token
    assert params.provider.name == 'Balancer'
    assert params.expected_profit > 0

@pytest.mark.asyncio
async def test_balancer_multi_token_flash_loan(flash_loan_manager):
    """Test multi-token flash loan from Balancer"""
    amounts = [int(1e18), int(1e18)]  # 1 ETH, 1 WBTC
    tokens = [
        Address('0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2'),  # WETH
        Address('0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599')   # WBTC
    ]
    route = tokens + [tokens[0]]  # Create a cycle
    
    params = await flash_loan_manager.prepare_loan(amounts[0], tokens[0], route)
    assert params is not None
    assert params.provider.name == 'Balancer'
    assert params.expected_profit > 0

@pytest.mark.asyncio
async def test_balancer_gas_estimation(flash_loan_manager):
    """Test gas cost estimation for Balancer flash loans"""
    amount = int(1e18)  # 1 ETH
    token = Address('0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2')  # WETH
    provider = flash_loan_manager.providers.get('balancer')
    
    gas_cost = flash_loan_manager._estimate_gas_cost(provider, amount)
    assert gas_cost >= 0
    assert isinstance(gas_cost, int)
