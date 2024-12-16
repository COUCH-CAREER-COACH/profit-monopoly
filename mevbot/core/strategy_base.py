"""Base Strategy with Optimizations"""
from abc import ABC, abstractmethod
import numpy as np
from typing import Optional, List, Dict, Tuple, Any
import logging
import asyncio
from web3 import Web3

from .utils.math import analyze_opportunities_vectorized, optimize_gas_vectorized

logger = logging.getLogger(__name__)

class BaseStrategy(ABC):
    """Base class for MEV strategies with optimizations"""
    
    def __init__(self, config: Dict, web3: Web3):
        """Initialize strategy with configuration."""
        self.config = config
        self.web3 = web3
        self.strategy_id = config.get('strategy_id', 'unknown')
        self.running = False
        self.last_execution = 0
        
    def get_id(self) -> str:
        """Get strategy identifier."""
        return self.strategy_id
        
    async def start(self) -> None:
        """Start the strategy."""
        if self.running:
            logger.warning(f"Strategy {self.strategy_id} already running")
            return
            
        logger.info(f"Starting strategy {self.strategy_id}")
        self.running = True
        
    async def stop(self) -> None:
        """Stop the strategy."""
        if not self.running:
            return
            
        logger.info(f"Stopping strategy {self.strategy_id}")
        self.running = False
        
    def is_ready(self) -> bool:
        """Check if strategy is ready to execute."""
        if not self.running:
            return False
            
        # Check cooldown period
        cooldown = self.config.get('execution_cooldown', 1.0)  # seconds
        current_time = asyncio.get_event_loop().time()
        if current_time - self.last_execution < cooldown:
            return False
            
        return True
        
    @abstractmethod
    async def execute(self) -> Optional[Dict[str, Any]]:
        """
        Execute the strategy.
        
        Returns:
            Optional transaction dict if an opportunity is found
        """
        pass
        
    async def analyze_market_data(self, market_data: Dict[str, np.ndarray]) -> Tuple[bool, float]:
        """
        Analyze market data for opportunities.
        
        Args:
            market_data: Dict containing market data arrays
            
        Returns:
            Tuple of (opportunity exists, expected profit)
        """
        try:
            min_profit = self.config.get('min_profit', 0)
            return analyze_opportunities_vectorized(market_data, min_profit)
            
        except Exception as e:
            logger.error(f"Error analyzing market data: {e}")
            return False, 0.0
            
    def optimize_gas(self, gas_prices: np.ndarray, success_rates: np.ndarray) -> int:
        """
        Optimize gas price for transaction.
        
        Args:
            gas_prices: Array of potential gas prices
            success_rates: Array of historical success rates
            
        Returns:
            Optimal gas price
        """
        try:
            return optimize_gas_vectorized(gas_prices, success_rates)
        except Exception as e:
            logger.error(f"Error optimizing gas: {e}")
            return self.config.get('default_gas_price', 50)  # gwei
