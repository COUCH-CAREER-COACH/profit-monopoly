"""Performance Monitoring System"""
import time
import numpy as np
from typing import Dict, List, Optional
import logging
import psutil
import asyncio
from collections import deque
from dataclasses import dataclass
from datetime import datetime, timedelta
from contextlib import asynccontextmanager

logger = logging.getLogger(__name__)

@dataclass
class LatencyMetrics:
    rpc_calls: deque
    tx_submissions: deque
    calculations: deque
    pool_updates: deque
    
    def __init__(self, maxlen: int = 1000):
        self.rpc_calls = deque(maxlen=maxlen)
        self.tx_submissions = deque(maxlen=maxlen)
        self.calculations = deque(maxlen=maxlen)
        self.pool_updates = deque(maxlen=maxlen)

@dataclass
class SystemMetrics:
    cpu_usage: float
    memory_usage: float
    network_io: Dict[str, int]
    disk_io: Dict[str, int]
    timestamp: datetime

class PerformanceMonitor:
    """Monitor and track performance metrics"""
    
    def __init__(self, config: Dict):
        self.config = config
        self.success_rates = {}  # Store success rates for different operations
        self.latencies = {}  # Store latencies for different operations
        self.profits = []  # Store profit history
        self.gas_costs = []  # Store gas cost history
        
        # Performance thresholds - more lenient in test mode
        self.thresholds = {
            'max_rpc_latency': 0.5 if config.get('test_mode') else 0.1,  # 500ms in test, 100ms in prod
            'max_calc_latency': 0.5 if config.get('test_mode') else 0.05,  # 500ms in test, 50ms in prod
            'min_success_rate': 0.5 if config.get('test_mode') else 0.95,  # 50% in test, 95% in prod
            'max_memory_usage': 0.9,  # 90%
            'min_profit': 0.01 if config.get('test_mode') else 0.05  # 0.01 ETH in test, 0.05 ETH in prod
        }
    
    @asynccontextmanager
    async def measure_latency(self, metric: str):
        """Context manager to measure latency of operations"""
        start_time = time.time()
        try:
            yield
        finally:
            duration = time.time() - start_time
            if metric not in self.latencies:
                self.latencies[metric] = []
            
            # Add new latency
            self.latencies[metric].append(duration)
            
            # Keep only last 100 measurements
            if len(self.latencies[metric]) > 100:
                self.latencies[metric] = self.latencies[metric][-100:]
            
            # Calculate average latency
            avg_latency = sum(self.latencies[metric]) / len(self.latencies[metric])
            
            # Log warning if latency is high (adjusted for test mode)
            threshold = 0.5 if self.config.get('test_mode') else 0.1  # More lenient in test mode
            if avg_latency > threshold:
                logger.warning(f"High {metric} latency detected: {avg_latency:.3f}s")
    
    async def collect_system_metrics(self):
        """Collect system performance metrics"""
        try:
            cpu = psutil.cpu_percent(interval=1) / 100
            memory = psutil.virtual_memory().percent / 100
            net_io = psutil.net_io_counters()._asdict()
            disk_io = psutil.disk_io_counters()._asdict()
            
            metrics = SystemMetrics(
                cpu_usage=cpu,
                memory_usage=memory,
                network_io=net_io,
                disk_io=disk_io,
                timestamp=datetime.now()
            )
            
            # Check thresholds
            if cpu > self.thresholds['max_cpu_usage']:
                logger.warning(f"High CPU usage: {cpu*100:.1f}%")
            if memory > self.thresholds['max_memory_usage']:
                logger.warning(f"High memory usage: {memory*100:.1f}%")
                
        except Exception as e:
            logger.error(f"Error collecting system metrics: {e}")
    
    def update_success_rate(self, metric: str, success: bool) -> None:
        """Update success rate for a given metric"""
        if metric not in self.success_rates:
            self.success_rates[metric] = []
        
        # Add new result
        self.success_rates[metric].append(success)
        
        # Keep only last 100 results
        if len(self.success_rates[metric]) > 100:
            self.success_rates[metric] = self.success_rates[metric][-100:]
        
        # Calculate success rate
        rate = sum(self.success_rates[metric]) / len(self.success_rates[metric])
        
        # Log warning if success rate is low (more lenient in test mode)
        threshold = 0.5 if self.config.get('test_mode') else 0.95
        if rate < threshold:
            logger.warning(f"Low success rate for {metric}: {rate*100:.1f}%")
    
    def record_profit(self, net_profit: float, gas_cost: float) -> None:
        """Record profit from an executed opportunity"""
        try:
            # Add to profit history
            self.profits.append(net_profit)
            self.gas_costs.append(gas_cost)
            
            # Keep only last 100 records
            if len(self.profits) > 100:
                self.profits = self.profits[-100:]
                self.gas_costs = self.gas_costs[-100:]
            
            # Calculate total profits
            total_profit = sum(self.profits)
            total_gas = sum(self.gas_costs) / 1e18  # Convert to ETH
            
            # Log significant profits
            threshold = 0.01 if self.config.get('test_mode') else 0.05  # Lower threshold in test mode
            if net_profit > threshold:
                logger.info(f"Significant profit: {net_profit:.4f} ETH (gas: {gas_cost/1e18:.4f} ETH)")
            
        except Exception as e:
            logger.error(f"Error recording profit: {e}")
    
    def get_performance_report(self) -> Dict:
        """Generate comprehensive performance report"""
        try:
            # Calculate metrics
            rpc_latency = np.mean(list(self.latencies.get('rpc_calls', []))) if self.latencies.get('rpc_calls', []) else 0
            calc_latency = np.mean(list(self.latencies.get('calculations', []))) if self.latencies.get('calculations', []) else 0
            success_rates = {
                k: np.mean(list(v)) if v else 0
                for k, v in self.success_rates.items()
            }
            
            # Calculate profit metrics
            profits = list(self.profits)
            total_profit = sum(profits)
            avg_profit = np.mean(profits) if profits else 0
            profit_std = np.std(profits) if len(profits) > 1 else 0
            
            return {
                'latency': {
                    'rpc_calls': f"{rpc_latency*1000:.2f}ms",
                    'calculations': f"{calc_latency*1000:.2f}ms",
                },
                'success_rates': {
                    k: f"{v:.1%}"
                    for k, v in success_rates.items()
                },
                'profit': {
                    'total': f"{total_profit:.4f} ETH",
                    'average': f"{avg_profit:.4f} ETH",
                    'std_dev': f"{profit_std:.4f} ETH",
                },
                'timestamp': datetime.now().isoformat()
            }
            
        except Exception as e:
            logger.error(f"Error generating performance report: {e}")
            return {}
    
    async def monitor_loop(self):
        """Continuous monitoring loop"""
        while True:
            try:
                await self.collect_system_metrics()
                
                # Generate and log report every minute
                if datetime.now().second == 0:
                    report = self.get_performance_report()
                    logger.info(f"Performance Report: {report}")
                
                await asyncio.sleep(1)
                
            except Exception as e:
                logger.error(f"Error in monitoring loop: {e}")
                await asyncio.sleep(5)
