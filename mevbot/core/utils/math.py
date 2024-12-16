"""Math utilities for MEV calculations."""
import numpy as np
from typing import Dict, Tuple

def calculate_profits_vectorized(
    prices: np.ndarray,
    amounts: np.ndarray,
    fees: np.ndarray
) -> np.ndarray:
    """
    Calculate profits using vectorized operations.
    
    Args:
        prices: Array of asset prices
        amounts: Array of transaction amounts
        fees: Array of transaction fees
        
    Returns:
        Array of potential profits
    """
    if len(prices) == 0 or len(amounts) == 0 or len(fees) == 0:
        raise ValueError("Input arrays cannot be empty")
    
    if not (len(prices) == len(amounts) == len(fees)):
        raise ValueError("All input arrays must have the same length")
        
    # Calculate raw profits
    raw_profits = np.multiply(prices, amounts)
    
    # Subtract fees
    net_profits = raw_profits - fees
    
    return net_profits

def optimize_gas_vectorized(
    gas_prices: np.ndarray,
    success_rates: np.ndarray
) -> int:
    """
    Optimize gas price using vectorized operations.
    
    Args:
        gas_prices: Array of potential gas prices
        success_rates: Array of historical success rates
        
    Returns:
        Optimal gas price
    """
    if np.any(gas_prices < 0):
        raise ValueError("Gas prices cannot be negative")
        
    if np.any((success_rates < 0) | (success_rates > 1)):
        raise ValueError("Success rates must be between 0 and 1")
    
    # Find index with highest success rate
    optimal_index = np.argmax(success_rates)
    
    return int(gas_prices[optimal_index])

def analyze_opportunities_vectorized(
    market_data: Dict[str, np.ndarray],
    min_profit: float
) -> Dict[str, np.ndarray]:
    """
    Analyze opportunities using SIMD operations.
    
    Args:
        market_data: Dict containing market data arrays
        min_profit: Minimum profit threshold
        
    Returns:
        Dict containing profitable indices and expected profits
    """
    # Extract market data
    prices = market_data['prices']
    amounts = market_data['amounts']  
    fees = market_data['fees']
    
    # Calculate profits
    profits = calculate_profits_vectorized(prices, amounts, fees)
    
    # Find profitable opportunities
    profitable_indices = np.where(profits >= min_profit)[0]
    
    return {
        'profitable_indices': profitable_indices,
        'expected_profits': profits
    }
