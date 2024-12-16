"""Tests for performance metrics"""
import pytest
from mevbot.core.metrics import PerformanceTracker, Timer, StrategyMetrics
import logging
import time

@pytest.fixture(autouse=True)
def setup_logging(caplog):
    """Set up logging for tests"""
    caplog.set_level(logging.INFO)

def test_strategy_metrics():
    """Test strategy metrics calculations"""
    metrics = StrategyMetrics(name="test")
    
    # Add some test data
    metrics.attempts = 100
    metrics.successes = 75
    metrics.failures = 25
    metrics.total_profit = 10.0
    metrics.total_gas = 1000000
    metrics.execution_times = [10.0, 20.0, 30.0]
    
    # Test calculations
    assert metrics.success_rate == 0.75
    assert metrics.average_profit == pytest.approx(0.133333, rel=1e-5)
    assert metrics.average_gas == 10000
    assert metrics.average_execution_time == 20.0

def test_performance_tracker():
    """Test performance tracker"""
    tracker = PerformanceTracker()
    
    # Track some executions
    tracker.track_execution(
        strategy="sandwich",
        success=True,
        profit=1.0,
        gas_used=100000,
        execution_time=15.0
    )
    
    tracker.track_execution(
        strategy="sandwich",
        success=False,
        profit=0.0,
        gas_used=50000,
        execution_time=10.0
    )
    
    # Get metrics
    metrics = tracker.get_metrics("sandwich")
    assert metrics is not None
    assert metrics.attempts == 2
    assert metrics.successes == 1
    assert metrics.failures == 1
    assert metrics.total_profit == 1.0
    assert metrics.total_gas == 150000
    assert len(metrics.execution_times) == 2

def test_timer():
    """Test timer context manager"""
    timer = Timer()
    
    with timer:
        time.sleep(0.01)  # Sleep for 10ms
    
    assert timer.duration >= 10  # Should be at least 10ms
    assert timer.duration < 100  # But not too long

def test_multiple_strategies():
    """Test tracking multiple strategies"""
    tracker = PerformanceTracker()
    
    # Track different strategies
    strategies = ["sandwich", "arbitrage", "liquidation"]
    
    for strategy in strategies:
        tracker.track_execution(
            strategy=strategy,
            success=True,
            profit=1.0,
            gas_used=100000,
            execution_time=15.0
        )
    
    # Get all metrics
    all_metrics = tracker.get_all_metrics()
    
    assert len(all_metrics) == 3
    for strategy in strategies:
        assert strategy in all_metrics
        metrics = all_metrics[strategy]
        assert metrics.attempts == 1
        assert metrics.successes == 1
        assert metrics.total_profit == 1.0

def test_print_summary(caplog):
    """Test metrics summary printing"""
    tracker = PerformanceTracker()

    # Add some test data
    tracker.track_execution(
        strategy="test",
        success=True,
        profit=1.0,
        gas_used=100000,
        execution_time=15.0
    )

    # Print summary
    tracker.print_summary()

    # Check log output
    assert "Strategy: test" in caplog.text
    assert "Success Rate: 100.00%" in caplog.text
    assert "Total Profit: 1.0000 ETH" in caplog.text
    assert "Average Profit: 1.0000 ETH" in caplog.text
    assert "Average Gas: 100000" in caplog.text
    assert "Average Execution Time: 15.00ms" in caplog.text
