"""Tests for SIMD operations"""
import pytest
import numpy as np
from mevbot.core.simd import SIMDProcessor

def test_batch_price_impact():
    """Test SIMD price impact calculations"""
    processor = SIMDProcessor()
    
    # Test data
    amounts = [1e18, 2e18, 5e18]  # 1, 2, 5 ETH
    liquidities = [1e20, 1e20, 1e20]  # 100 ETH each
    
    impacts = processor.batch_price_impact(amounts, liquidities)
    
    assert len(impacts) == 3
    assert all(0 < impact < 1 for impact in impacts)
    assert impacts[0] < impacts[1] < impacts[2]  # Higher amounts = higher impact

def test_batch_profitability():
    """Test SIMD profitability calculations"""
    processor = SIMDProcessor()
    
    # Test data
    buy_prices = [1000, 2000, 3000]
    sell_prices = [1100, 2200, 3300]
    gas_costs = [50, 50, 50]
    
    profits = processor.batch_profitability(buy_prices, sell_prices, gas_costs)
    
    assert len(profits) == 3
    assert all(profit > 0 for profit in profits)
    assert profits[0] < profits[1] < profits[2]

def test_batch_sandwich_optimization():
    """Test SIMD sandwich attack optimization"""
    processor = SIMDProcessor()
    
    # Test data
    target_amounts = [1e18, 2e18, 5e18]  # Target transaction amounts
    pool_liquidities = [1e20, 1e20, 1e20]  # Pool liquidities
    gas_prices = [50e9, 50e9, 50e9]  # 50 gwei
    
    amounts, profits = processor.batch_sandwich_optimization(
        target_amounts,
        pool_liquidities,
        gas_prices
    )
    
    assert len(amounts) == len(profits) == 3
    assert all(amount >= 0 for amount in amounts)
    assert all(profit >= 0 for profit in profits)

def test_simd_performance():
    """Benchmark SIMD operations"""
    processor = SIMDProcessor()
    
    # Generate large test data
    size = 1000
    amounts = np.random.uniform(1e18, 10e18, size)
    liquidities = np.full(size, 1e20)
    gas_prices = np.full(size, 50e9)
    
    # Time the operations
    import time
    
    start = time.perf_counter()
    amounts, profits = processor.batch_sandwich_optimization(
        amounts,
        liquidities,
        gas_prices
    )
    duration = (time.perf_counter() - start) * 1e6  # Convert to microseconds
    
    print(f"\nSIMD Performance (1000 items):")
    print(f"Sandwich optimization: {duration:.2f}μs")
    
    assert duration < 15000  # Should be under 15ms (15000μs)
