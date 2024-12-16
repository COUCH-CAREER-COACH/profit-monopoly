"""Performance tracking and metrics collection"""
from typing import Dict, List, Optional
from dataclasses import dataclass, field
from collections import defaultdict
import time
import logging
import numpy as np

logger = logging.getLogger(__name__)

@dataclass
class StrategyMetrics:
    name: str
    attempts: int = 0
    successes: int = 0
    failures: int = 0
    total_profit: float = 0.0
    total_gas: int = 0
    execution_times: List[float] = field(default_factory=list)
    
    @property
    def success_rate(self) -> float:
        """Calculate success rate"""
        return self.successes / self.attempts if self.attempts > 0 else 0
        
    @property
    def average_profit(self) -> float:
        """Calculate average profit per successful trade"""
        return self.total_profit / self.successes if self.successes > 0 else 0
        
    @property
    def average_gas(self) -> float:
        """Calculate average gas per attempt"""
        return self.total_gas / self.attempts if self.attempts > 0 else 0
        
    @property
    def average_execution_time(self) -> float:
        """Calculate average execution time"""
        return np.mean(self.execution_times) if self.execution_times else 0

class PerformanceTracker:
    def __init__(self):
        self.metrics: Dict[str, StrategyMetrics] = defaultdict(
            lambda: StrategyMetrics(name="unknown")
        )
        
    def track_execution(
        self,
        strategy: str,
        success: bool,
        profit: float,
        gas_used: int,
        execution_time: float
    ):
        """Track strategy execution"""
        metrics = self.metrics[strategy]
        metrics.name = strategy
        metrics.attempts += 1
        
        if success:
            metrics.successes += 1
            metrics.total_profit += profit
        else:
            metrics.failures += 1
            
        metrics.total_gas += gas_used
        metrics.execution_times.append(execution_time)
        
    def get_metrics(self, strategy: str) -> Optional[StrategyMetrics]:
        """Get metrics for a specific strategy"""
        return self.metrics.get(strategy)
        
    def get_all_metrics(self) -> Dict[str, StrategyMetrics]:
        """Get metrics for all strategies"""
        return dict(self.metrics)
        
    def print_summary(self):
        """Print summary of all metrics"""
        for strategy, metrics in self.metrics.items():
            logger.info(f"\nStrategy: {strategy}")
            logger.info(f"Success Rate: {metrics.success_rate:.2%}")
            logger.info(f"Total Profit: {metrics.total_profit:.4f} ETH")
            logger.info(f"Average Profit: {metrics.average_profit:.4f} ETH")
            logger.info(f"Average Gas: {metrics.average_gas:.0f}")
            logger.info(f"Average Execution Time: {metrics.average_execution_time:.2f}ms")
            
class Timer:
    """Context manager for timing operations"""
    def __init__(self):
        self.start_time = None
        self.end_time = None
        
    def __enter__(self):
        self.start_time = time.perf_counter()
        return self
        
    def __exit__(self, *args):
        self.end_time = time.perf_counter()
        
    @property
    def duration(self) -> float:
        """Get duration in milliseconds"""
        if self.start_time and self.end_time:
            return (self.end_time - self.start_time) * 1000
        return 0.0
