import pytest
import numpy as np
from web3 import Web3
from mevbot.core.strategy import MEVStrategy
import os
from dotenv import load_dotenv
from eth_typing import Address

# Load environment variables
load_dotenv()

@pytest.fixture
def w3():
    return Web3(Web3.HTTPProvider(os.getenv('HTTPS_URL_SEPOLIA')))

@pytest.fixture
def config():
    return {
        'fee_percentage': 0.003,
        'min_profit': 0.1,
        'max_slippage': 0.005
    }

@pytest.fixture
def strategy(w3, config):
    """Create strategy for testing"""
    return MEVStrategy(w3, config)

def test_calculate_profit_simd(strategy):
    """Test SIMD profit calculation"""
    # Test data
    path = [
        Address('0x' + '1' * 40),
        Address('0x' + '2' * 40),
        Address('0x' + '3' * 40)
    ]
    amounts = np.array([1e18, 2e18, 3e18])  # 1-3 ETH
    pool_states = {
        path[0]: {'depth': 1000e18},
        path[1]: {'depth': 2000e18}
    }
    
    # Calculate profit
    profits, slippage = strategy.calculate_profit(path, amounts, pool_states)
    
    assert len(profits) == len(amounts)
    assert len(slippage) == len(amounts)
    assert all(isinstance(p, float) for p in profits)
    assert all(isinstance(s, float) for s in slippage)
    assert all(s >= 0 for s in slippage)  # Slippage should be non-negative

def test_calculate_slippage_vectorized(strategy):
    """Test vectorized slippage calculation"""
    # Test data
    path = [
        Address('0x' + '1' * 40),
        Address('0x' + '2' * 40),
        Address('0x' + '3' * 40)
    ]
    amounts = np.array([1e18, 2e18, 3e18])
    pool_states = {
        path[0]: {'depth': 1000e18},
        path[1]: {'depth': 2000e18}
    }
    
    # Calculate slippage
    slippage = strategy._calculate_slippage_vectorized(amounts, pool_states, path)
    
    assert len(slippage) == len(amounts)
    assert all(isinstance(s, float) for s in slippage)
    assert all(s >= 0 for s in slippage)
    assert slippage[0] < slippage[1] < slippage[2]  # Higher amounts = higher slippage

def test_calculate_success_probability(strategy):
    """Test success probability calculation"""
    # Test different network conditions
    probabilities = [
        strategy.calculate_success_probability(
            gas_price=gas_price,
            network_congestion=congestion,
            competitor_count=competitors
        )
        for gas_price, congestion, competitors in [
            (50e9, 0.5, 5),   # Medium conditions
            (100e9, 0.8, 10),  # High competition
            (20e9, 0.2, 2)     # Low competition
        ]
    ]
    
    assert all(0 <= p <= 1 for p in probabilities)
    assert probabilities[1] < probabilities[0] < probabilities[2]

def test_network_congestion(strategy):
    """Test network congestion analysis"""
    conditions = strategy.analyze_network_conditions()
    
    assert isinstance(conditions, dict)
    if conditions:  # If connected to network
        assert 'congestion' in conditions
        assert 'gas_price' in conditions
        assert 'block_number' in conditions
        assert 0 <= conditions['congestion'] <= 1

def test_competition_analysis(strategy):
    """Test competition analysis"""
    # Test data
    path = [
        Address('0x' + '1' * 40),
        Address('0x' + '2' * 40)
    ]
    amounts = np.array([1e18])
    pool_states = {
        path[0]: {'depth': 1000e18}
    }
    
    # Calculate profit with different competition levels
    profits1, _ = strategy.calculate_profit(path, amounts, pool_states)
    
    # Simulate higher competition by reducing pool depth
    pool_states[path[0]]['depth'] = 500e18
    profits2, _ = strategy.calculate_profit(path, amounts, pool_states)
    
    assert profits1[0] > profits2[0]  # Higher competition = lower profits
