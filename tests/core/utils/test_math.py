"""Tests for math utility functions."""
import pytest
import numpy as np
from mevbot.core.utils.math import (
    calculate_profits_vectorized,
    optimize_gas_vectorized,
    analyze_opportunities_vectorized
)

@pytest.fixture
def sample_data():
    """Sample market data for testing."""
    return {
        'prices': np.array([100.0, 101.0, 99.5, 102.0, 98.0]),
        'amounts': np.array([1000.0, 1500.0, 800.0, 2000.0, 500.0]),
        'fees': np.array([10.0, 15.0, 8.0, 20.0, 5.0]),
        'gas_prices': np.array([40.0, 50.0, 60.0, 70.0, 80.0]),
        'success_rates': np.array([0.95, 0.97, 0.98, 0.99, 0.995])
    }

def test_calculate_profits_vectorized(sample_data):
    """Test vectorized profit calculations."""
    profits = calculate_profits_vectorized(
        sample_data['prices'],
        sample_data['amounts'],
        sample_data['fees']
    )
    
    assert isinstance(profits, np.ndarray)
    assert len(profits) == len(sample_data['prices'])
    assert np.all(np.isfinite(profits))  # Profits should be finite
    
    # Test specific profit calculation
    expected_profit = 100.0 * 1000.0 - 10.0  # First transaction
    assert profits[0] == pytest.approx(expected_profit, rel=1e-5)

def test_optimize_gas_vectorized(sample_data):
    """Test gas price optimization."""
    optimal_gas = optimize_gas_vectorized(
        sample_data['gas_prices'],
        sample_data['success_rates']
    )
    
    assert isinstance(optimal_gas, int)
    assert optimal_gas in sample_data['gas_prices']
    
    # Higher success rate should be preferred
    best_success_idx = np.argmax(sample_data['success_rates'])
    assert optimal_gas == sample_data['gas_prices'][best_success_idx]

def test_analyze_opportunities_vectorized(sample_data):
    """Test opportunity analysis."""
    min_profit = 1000.0
    opportunities = analyze_opportunities_vectorized(
        {
            'prices': sample_data['prices'],
            'amounts': sample_data['amounts'],
            'fees': sample_data['fees']
        },
        min_profit
    )
    
    assert isinstance(opportunities, dict)
    assert 'profitable_indices' in opportunities
    assert 'expected_profits' in opportunities
    
    # Verify profitable opportunities
    profits = opportunities['expected_profits']
    assert np.all(profits[opportunities['profitable_indices']] >= min_profit)

def test_edge_cases():
    """Test edge cases and error handling."""
    # Test empty arrays
    empty_arr = np.array([])
    with pytest.raises(ValueError):
        calculate_profits_vectorized(empty_arr, empty_arr, empty_arr)
    
    # Test arrays of different lengths
    prices = np.array([100.0, 101.0])
    amounts = np.array([1000.0])
    fees = np.array([10.0])
    with pytest.raises(ValueError):
        calculate_profits_vectorized(prices, amounts, fees)
    
    # Test negative values
    gas_prices = np.array([-50.0, 60.0, 70.0])
    success_rates = np.array([0.95, 0.97, 0.98])
    with pytest.raises(ValueError):
        optimize_gas_vectorized(gas_prices, success_rates)
    
    # Test invalid success rates
    gas_prices = np.array([50.0, 60.0, 70.0])
    success_rates = np.array([0.95, 1.5, 0.98])
    with pytest.raises(ValueError):
        optimize_gas_vectorized(gas_prices, success_rates)
