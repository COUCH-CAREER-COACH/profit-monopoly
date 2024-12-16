"""Circuit breaker system for MEV bot safety controls."""
import time
import logging
from typing import Dict, Optional, List
from decimal import Decimal
from dataclasses import dataclass
from datetime import datetime, timedelta
import asyncio
from web3 import Web3

logger = logging.getLogger(__name__)

@dataclass
class PositionLimit:
    max_size: Decimal
    current_size: Decimal = Decimal('0')

@dataclass
class GasLimit:
    max_price_gwei: int
    max_daily_spend: Decimal
    current_daily_spend: Decimal = Decimal('0')

@dataclass
class ProfitMetrics:
    min_profit_threshold: Decimal
    max_daily_loss: Decimal
    current_daily_pnl: Decimal = Decimal('0')

class CircuitBreaker:
    """
    Implements safety controls and circuit breakers for MEV operations.
    
    Features:
    - Position size limits
    - Gas price and spend limits
    - Profit/loss thresholds
    - Transaction rate limiting
    - Slippage protection
    """
    
    def __init__(self, config: Dict):
        """Initialize circuit breaker with configuration."""
        self.config = config
        self.enabled = True
        self.triggered = False
        self.trigger_reason = None
        
        # Initialize limits
        self.position_limit = PositionLimit(
            max_size=Decimal(str(config['max_position_size'])),
        )
        
        self.gas_limit = GasLimit(
            max_price_gwei=config['max_gas_price_gwei'],
            max_daily_spend=Decimal(str(config['max_daily_gas_spend'])),
        )
        
        self.profit_metrics = ProfitMetrics(
            min_profit_threshold=Decimal(str(config['min_profit_threshold'])),
            max_daily_loss=Decimal(str(config['max_daily_loss'])),
        )
        
        # Transaction rate limiting
        self.tx_window = float(config.get('tx_rate_window', 60))  # 60 seconds default
        self.max_tx_per_window = config.get('max_tx_per_window', 10)
        self.tx_timestamps: List[float] = []
        
        # Slippage protection
        self.max_slippage = Decimal(str(config.get('max_slippage', '0.01')))  # 1% default
        
        # Last check time
        self.last_health_check = time.time()
        self.last_metrics_reset = time.time()
    
    async def validate_transaction(self, tx: Dict) -> bool:
        """Validate if a transaction meets all safety criteria."""
        if not self.enabled:
            logger.warning("Circuit breaker is disabled")
            return False
            
        try:
            # Check system health
            await self._check_system_health()
            
            # Check if breaker is triggered
            if self.triggered:
                logger.warning("Circuit breaker is triggered")
                return False
            
            # Get transaction details
            value = Decimal(str(Web3.from_wei(tx['value'], 'ether')))
            gas_price = Decimal(str(Web3.from_wei(tx.get('gasPrice', 0), 'gwei'))) if 'gasPrice' in tx else Decimal('0')
            gas = Decimal(str(tx.get('gas', 21000)))  # Default gas limit
            expected_profit = Decimal(str(Web3.from_wei(tx.get('expected_profit', 0), 'ether')))
            
            # Check position size
            if not await self._check_position_size(value):
                return False
                
            # Check gas limits
            if not await self._check_gas_limits(gas_price, gas):
                return False
                
            # Check transaction rate
            if not await self._check_tx_rate():
                return False
                
            # Check profit threshold
            if not await self._check_profit_threshold(expected_profit):
                return False
                
            # Check slippage if applicable
            if 'expected_price' in tx and 'actual_price' in tx:
                expected_price = Decimal(str(Web3.from_wei(tx['expected_price'], 'ether')))
                actual_price = Decimal(str(Web3.from_wei(tx['actual_price'], 'ether')))
                if not await self._check_slippage(expected_price, actual_price):
                    return False
            
            # All checks passed, record transaction
            self.tx_timestamps.append(time.time())
            self.position_limit.current_size += value
            
            return True
            
        except Exception as e:
            logger.error(f"Transaction validation failed: {str(e)}")
            return False
    
    async def _check_position_size(self, value: Decimal) -> bool:
        """Check if position size is within limits."""
        if self.position_limit.current_size + value > self.position_limit.max_size:
            await self.trigger_breaker(f"Position size limit exceeded: {self.position_limit.current_size + value} > {self.position_limit.max_size}")
            return False
        return True
    
    async def _check_gas_limits(self, gas_price: Decimal, gas: Decimal) -> bool:
        """Check if gas price and total spend are within limits."""
        if gas_price > self.gas_limit.max_price_gwei:
            await self.trigger_breaker(f"Gas price too high: {gas_price} > {self.gas_limit.max_price_gwei}")
            return False
            
        gas_cost = gas_price * gas / Decimal('1000000000')  # Convert to ETH
        if self.gas_limit.current_daily_spend + gas_cost > self.gas_limit.max_daily_spend:
            await self.trigger_breaker("Daily gas spend limit would be exceeded")
            return False
            
        self.gas_limit.current_daily_spend += gas_cost
        return True
    
    async def _check_tx_rate(self) -> bool:
        """Check if transaction rate is within limits."""
        current_time = time.time()
        
        # Remove expired timestamps
        self.tx_timestamps = [t for t in self.tx_timestamps if current_time - t <= self.tx_window]
        
        # Check if we would exceed the rate limit
        if len(self.tx_timestamps) >= self.max_tx_per_window:
            logger.warning(f"Transaction rate limit reached: {len(self.tx_timestamps)} >= {self.max_tx_per_window}")
            return False
            
        return True
    
    async def _check_profit_threshold(self, expected_profit: Decimal) -> bool:
        """Check if expected profit meets minimum threshold."""
        if expected_profit < self.profit_metrics.min_profit_threshold:
            await self.trigger_breaker(f"Profit below minimum threshold: {expected_profit} < {self.profit_metrics.min_profit_threshold}")
            return False
        return True
    
    async def _check_slippage(self, expected_price: Decimal, actual_price: Decimal) -> bool:
        """Check if price slippage is within acceptable range."""
        slippage = abs(actual_price - expected_price) / expected_price
        if slippage > self.max_slippage:
            await self.trigger_breaker(f"Slippage too high: {slippage} > {self.max_slippage}")
            return False
        return True
    
    async def record_profit_loss(self, amount: Decimal) -> None:
        """Record profit/loss from a transaction."""
        self.profit_metrics.current_daily_pnl += amount
        
        if self.profit_metrics.current_daily_pnl <= -self.profit_metrics.max_daily_loss:
            await self.trigger_breaker("Maximum daily loss exceeded")
    
    async def trigger_breaker(self, reason: str) -> None:
        """Trigger the circuit breaker."""
        logger.warning(f"Circuit breaker triggered: {reason}")
        self.triggered = True
        self.trigger_reason = reason
    
    async def reset_breaker(self) -> None:
        """Reset the circuit breaker after manual intervention."""
        logger.info("Resetting circuit breaker")
        self.triggered = False
        self.trigger_reason = None
        self.tx_timestamps.clear()
        self.position_limit.current_size = Decimal('0')
        self.gas_limit.current_daily_spend = Decimal('0')
        self.profit_metrics.current_daily_pnl = Decimal('0')
        self.last_metrics_reset = time.time()
    
    async def _check_system_health(self) -> None:
        """Check system health and reset metrics if needed."""
        current_time = time.time()
        
        # Check if we need to reset metrics
        if current_time - self.last_metrics_reset >= self.config.get('metrics_reset_interval', 86400):
            logger.info("Resetting daily metrics")
            # Reset metrics but preserve breaker state
            self.position_limit.current_size = Decimal('0')
            self.gas_limit.current_daily_spend = Decimal('0')
            self.profit_metrics.current_daily_pnl = Decimal('0')
            self.tx_timestamps.clear()
            self.last_metrics_reset = current_time
            return
        
        # Check if we need to do health check
        if current_time - self.last_health_check >= self.config.get('health_check_interval', 60):
            # Check position limits
            if self.position_limit.current_size > self.position_limit.max_size:
                await self.trigger_breaker("Position limit exceeded")
            
            # Check daily loss
            if self.profit_metrics.current_daily_pnl <= -self.profit_metrics.max_daily_loss:
                await self.trigger_breaker("Maximum daily loss exceeded")
            
            # Check gas spend
            if self.gas_limit.current_daily_spend > self.gas_limit.max_daily_spend:
                await self.trigger_breaker("Daily gas spend limit exceeded")
            
            self.last_health_check = current_time
    
    async def emergency_shutdown(self):
        """Trigger emergency shutdown."""
        await self.trigger_breaker("Emergency shutdown activated")

    def is_triggered(self) -> bool:
        """Check if circuit breaker has been triggered."""
        return self.triggered

    def get_trigger_reason(self) -> str:
        """Get the reason for circuit breaker triggering."""
        return self.trigger_reason
