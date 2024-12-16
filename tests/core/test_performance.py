"""Tests for Performance Monitoring System"""
import pytest
import asyncio
import numpy as np
from datetime import datetime
from unittest.mock import Mock, patch, AsyncMock
import time

from mevbot.core.performance import PerformanceMonitor, LatencyMetrics, SystemMetrics

@pytest.fixture
def config():
    return {
        'max_rpc_latency': 0.1,
        'max_calc_latency': 0.05,
        'min_success_rate': 0.95,
        'max_memory_usage': 0.9,
        'max_cpu_usage': 0.8,
    }

@pytest.fixture
def monitor(config):
    return PerformanceMonitor(config)

@pytest.mark.asyncio
async def test_latency_measurement(monitor):
    """Test latency measurement context manager"""
    async with monitor.measure_latency('rpc_calls'):
        await asyncio.sleep(0.01)  # Simulate RPC call
        
    assert len(monitor.latency.rpc_calls) == 1
    assert 0.005 <= monitor.latency.rpc_calls[0] <= 0.015  # Allow for some variance

@pytest.mark.asyncio
async def test_system_metrics_collection(monitor):
    """Test system metrics collection"""
    with patch('psutil.cpu_percent', return_value=50.0), \
         patch('psutil.virtual_memory') as mock_memory, \
         patch('psutil.net_io_counters') as mock_net, \
         patch('psutil.disk_io_counters') as mock_disk:
        
        # Configure mocks
        mock_memory.return_value.percent = 60.0
        mock_net.return_value._asdict.return_value = {
            'bytes_sent': 1000,
            'bytes_recv': 2000
        }
        mock_disk.return_value._asdict.return_value = {
            'read_bytes': 3000,
            'write_bytes': 4000
        }
        
        await monitor.collect_system_metrics()
        
        assert len(monitor.system_metrics) == 1
        metrics = monitor.system_metrics[0]
        assert metrics.cpu_usage == 0.5  # 50%
        assert metrics.memory_usage == 0.6  # 60%
        assert metrics.network_io['bytes_sent'] == 1000
        assert metrics.disk_io['read_bytes'] == 3000

def test_success_rate_tracking(monitor):
    """Test success rate tracking"""
    # Simulate some successes and failures
    for _ in range(8):
        monitor.update_success_rate('opportunities', True)
    for _ in range(2):
        monitor.update_success_rate('opportunities', False)
        
    rates = list(monitor.success_rates['opportunities'])
    assert len(rates) == 10
    assert sum(rates) == 8  # 8 successes
    assert np.mean(rates) == 0.8  # 80% success rate

def test_profit_recording(monitor):
    """Test profit recording"""
    profits = [0.1, 0.2, -0.05, 0.15]  # Some profits and losses
    gas_costs = [0.01, 0.01, 0.01, 0.01]
    
    for profit, gas in zip(profits, gas_costs):
        monitor.record_profit(profit, gas)
        
    assert len(monitor.profit_history) == 4
    assert sum(monitor.profit_history) == pytest.approx(0.36)  # Total profit minus gas

@pytest.mark.asyncio
async def test_performance_report_generation(monitor):
    """Test performance report generation"""
    # Add some test data
    async with monitor.measure_latency('rpc_calls'):
        await asyncio.sleep(0.01)
    
    monitor.update_success_rate('opportunities', True)
    monitor.record_profit(0.1, 0.01)
    
    await monitor.collect_system_metrics()
    
    report = monitor.get_performance_report()
    
    assert 'latency' in report
    assert 'success_rates' in report
    assert 'profit' in report
    assert 'system' in report
    assert 'timestamp' in report
    
    # Verify report format
    assert isinstance(report['latency']['rpc_calls'], str)
    assert isinstance(report['success_rates']['opportunities'], str)
    assert isinstance(report['profit']['total'], str)
    assert isinstance(report['system']['cpu_usage'], str)
    assert isinstance(report['timestamp'], str)

@pytest.mark.asyncio
async def test_monitor_loop_execution(monitor):
    """Test monitor loop execution"""
    # Create a mock for collect_system_metrics
    monitor.collect_system_metrics = AsyncMock()
    
    # Run monitor loop for a short time
    task = asyncio.create_task(monitor.monitor_loop())
    await asyncio.sleep(2)  # Run for 2 seconds
    task.cancel()  # Stop the loop
    
    try:
        await task
    except asyncio.CancelledError:
        pass
    
    # Verify that collect_system_metrics was called
    assert monitor.collect_system_metrics.call_count >= 1

@pytest.mark.asyncio
async def test_high_latency_warning(monitor, caplog):
    """Test warning generation for high latency"""
    async with monitor.measure_latency('rpc_calls'):
        await asyncio.sleep(0.15)  # Exceed max_rpc_latency threshold
        
    assert any("High RPC latency detected" in record.message 
              for record in caplog.records)

@pytest.mark.asyncio
async def test_resource_usage_warning(monitor, caplog):
    """Test warning generation for high resource usage"""
    with patch('psutil.cpu_percent', return_value=90.0), \
         patch('psutil.virtual_memory') as mock_memory:
        
        mock_memory.return_value.percent = 95.0
        await monitor.collect_system_metrics()
        
    assert any("High CPU usage" in record.message 
              for record in caplog.records)
    assert any("High memory usage" in record.message 
              for record in caplog.records)
