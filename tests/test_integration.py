import pytest
import pytest_asyncio
from web3 import Web3
from eth_account import Account
from mevbot.core.flashbots import FlashbotsManager
from mevbot.core.flashloan import FlashLoanManager, FlashLoanProvider
from mevbot.core.mempool import MempoolManager
from mevbot.core.strategy import StrategyManager
import os
from dotenv import load_dotenv
import asyncio
from typing import List, Dict, Any

# Load environment variables
load_dotenv()

@pytest.fixture
def w3():
    """Create a Web3 instance"""
    return Web3(Web3.HTTPProvider('https://eth-mainnet.g.alchemy.com/v2/your-api-key'))

@pytest.fixture
def test_account():
    """Create a test account"""
    return Account.create()

@pytest.fixture
def config():
    return {
        'test_mode': True,  # Enable test mode
        'flashbots_relay_url': 'https://relay-sepolia.flashbots.net',
        'min_bid': 0.05,
        'max_bid': 2.0,
        'blocks_to_try': 3,
        'flash_loan_providers': ['aave', 'balancer'],
        'min_profit_threshold': 0.01,
        'max_gas_price': 100e9,
        'mempool_scan_interval': 1.0,
        'strategy_update_interval': 2.0,
        'aave_lending_pool': '0x7d2768dE32b0b80b7a3454c06BdAc94A69DDc7A9',  # Mainnet Aave
        'dydx_solo': '0x1E0447b19BB6EcFdAe1e4AE1694b0C3659614e4e',  # Mainnet dYdX
        'balancer_vault': '0xBA12222222228d8Ba445958a75a0704d566BF2C8',  # Mainnet Balancer
        'fee_percentage': 0.003  # 0.3% fee
    }

@pytest_asyncio.fixture
async def managers(config, test_account, w3):
    """Initialize managers for testing"""
    
    # Create managers
    flashbots_manager = FlashbotsManager(w3, test_account.key.hex(), config)
    flashloan_manager = FlashLoanManager(w3, config)
    mempool_manager = MempoolManager(w3, config)
    strategy_manager = StrategyManager(w3, config)

    # Initialize flash loan providers
    flashloan_manager.providers = {
        'aave': FlashLoanProvider(
            name='aave',
            address=config['aave_lending_pool'],
            max_loan_amount=int(1e20),  # 100 ETH for testing
            current_liquidity=int(1e20),
            fee_percentage=0.003
        ),
        'balancer': FlashLoanProvider(
            name='balancer',
            address=config['balancer_vault'],
            max_loan_amount=int(1e20),  # 100 ETH for testing
            current_liquidity=int(1e20),
            fee_percentage=0.003
        )
    }
    
    return {
        'flashbots': flashbots_manager,
        'flashloan': flashloan_manager,
        'mempool': mempool_manager,
        'strategy': strategy_manager
    }

@pytest.mark.asyncio
async def test_opportunity_detection_and_execution(managers):
    """Test the entire flow from opportunity detection to execution"""
    
    # 1. Monitor mempool for opportunities
    async def monitor_mempool():
        transactions = await managers['mempool'].scan_mempool()
        assert isinstance(transactions, list)
        return transactions
    
    # 2. Analyze opportunities with strategy manager
    async def analyze_opportunity(tx):
        metrics = await managers['strategy'].analyze_opportunity(tx)
        assert hasattr(metrics, 'profit')
        assert hasattr(metrics, 'required_loan')
        return metrics
    
    # 3. Prepare flash loan if needed
    async def prepare_flash_loan(amount, token):
        loan_params = await managers['flashloan'].prepare_loan(amount, token)
        assert loan_params is not None
        assert loan_params.amount > 0
        return loan_params
        
    # 4. Submit bundle to Flashbots
    async def submit_bundle(metrics, loan_params=None):
        bundle = await managers['flashbots'].create_bundle(metrics, loan_params)
        assert bundle is not None
        return bundle
        
    # Test flow
    transactions = await monitor_mempool()
    assert len(transactions) >= 0
    
    if transactions:
        metrics = await analyze_opportunity(transactions[0])
        assert metrics is not None
        
        if metrics.required_loan > 0:
            loan_params = await prepare_flash_loan(metrics.required_loan, metrics.token_address)
            assert loan_params is not None
            bundle = await submit_bundle(metrics, loan_params)
        else:
            bundle = await submit_bundle(metrics)
            
        assert bundle is not None

@pytest.mark.asyncio
async def test_parallel_opportunity_analysis(managers):
    """Test parallel analysis of multiple opportunities"""

    # Create mock transactions
    mock_txs = [
        {
            'hash': f'0x{i:064x}',  # Simple hex format
            'to': '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
            'value': (i + 1) * 1e18,  # Ensure non-zero value
            'gas': 21000,
            'maxFeePerGas': 2000000000,
            'maxPriorityFeePerGas': 1000000000,
            'nonce': i,
            'input': '0x'  # Empty input data for test
        }
        for i in range(5)
    ]

    # Analyze opportunities in parallel
    tasks = [
        managers['strategy'].analyze_opportunity(tx)
        for tx in mock_txs
    ]

    results = await asyncio.gather(*tasks)

    # Verify results
    assert len(results) == len(mock_txs)
    for metrics in results:
        assert metrics is not None
        assert hasattr(metrics, 'profit')
        assert metrics.profit >= 0
        assert metrics.gas_cost_usd >= 0
        assert 0 <= metrics.success_probability <= 1
        assert metrics.execution_time_ms >= 0
        assert len(metrics.path) > 0

@pytest.mark.asyncio
async def test_flash_loan_optimization(managers):
    """Test flash loan amount optimization"""
    
    # Test different loan amounts
    amounts = [1e18, 5e18, 10e18]  # 1 ETH, 5 ETH, 10 ETH
    token = '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2'  # WETH
    
    for amount in amounts:
        params = await managers['flashloan'].prepare_loan(int(amount), token)
        assert params is not None
        assert params.amount > 0
        assert params.expected_profit >= 0
        assert len(params.route) > 0

@pytest.mark.asyncio
async def test_mempool_filtering(managers):
    """Test mempool transaction filtering"""
    
    # Create mock transactions
    mock_txs = [
        {
            'hash': f'0x{i:064x}',  # Simple hex format
            'to': '0x742d35Cc6634C0532925a3b844Bc454e4438f44e',
            'value': (i + 1) * 1e18,  # Ensure non-zero value
            'gas': 21000,
            'maxFeePerGas': 2000000000,
            'maxPriorityFeePerGas': 1000000000,
            'nonce': i
        }
        for i in range(10)
    ]
    
    # Filter transactions
    filtered_txs = await managers['mempool'].filter_transactions(mock_txs)
    
    # Verify filtering
    assert len(filtered_txs) <= len(mock_txs)
    for tx in filtered_txs:
        assert tx['value'] > 0
        assert tx['gas'] <= managers['mempool'].config['max_gas_price']
