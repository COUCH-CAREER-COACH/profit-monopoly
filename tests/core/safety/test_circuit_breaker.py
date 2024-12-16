"""Test suite for circuit breaker functionality."""
import pytest
import pytest_asyncio
import time
import asyncio
from decimal import Decimal
from web3 import Web3

from mevbot.core.safety.circuit_breaker import CircuitBreaker

pytestmark = pytest.mark.asyncio

@pytest_asyncio.fixture(autouse=True)
async def circuit_breaker():
    """Create a circuit breaker instance for testing."""
    config = {
        'max_position_size': '100',
        'max_gas_price_gwei': 500,
        'max_daily_gas_spend': '10',
        'min_profit_threshold': '0.1',
        'max_daily_loss': '5',
        'tx_rate_window': 1,  # 1 second window for testing
        'max_tx_per_window': 2,
        'max_slippage': '0.01',
        'metrics_reset_interval': 1,  # 1 second for testing
        'health_check_interval': 0.1,  # 0.1 seconds for testing
    }
    breaker = CircuitBreaker(config)
    yield breaker
    await breaker.reset_breaker()

async def test_position_size_limit(circuit_breaker):
    """Test position size limits are enforced."""
    # Valid transaction
    tx1 = {
        'value': Web3.to_wei(50, 'ether'),
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert await circuit_breaker.validate_transaction(tx1)
    
    # Transaction that would exceed limit
    tx2 = {
        'value': Web3.to_wei(51, 'ether'),
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert not await circuit_breaker.validate_transaction(tx2)
    assert circuit_breaker.is_triggered()

async def test_gas_price_limit(circuit_breaker):
    """Test gas price limits are enforced."""
    # Valid transaction
    tx1 = {
        'value': Web3.to_wei(1, 'ether'),
        'gasPrice': Web3.to_wei(400, 'gwei'),
        'gas': 21000,
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert await circuit_breaker.validate_transaction(tx1)
    
    # Transaction with excessive gas price
    tx2 = {
        'value': Web3.to_wei(1, 'ether'),
        'gasPrice': Web3.to_wei(600, 'gwei'),
        'gas': 21000,
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert not await circuit_breaker.validate_transaction(tx2)
    assert circuit_breaker.is_triggered()

async def test_transaction_rate_limit(circuit_breaker):
    """Test transaction rate limiting."""
    # First two transactions should succeed
    tx = {
        'value': Web3.to_wei(1, 'ether'),
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert await circuit_breaker.validate_transaction(tx)
    assert await circuit_breaker.validate_transaction(tx)
    
    # Third transaction within window should fail
    assert not await circuit_breaker.validate_transaction(tx)
    
    # Wait for window to pass
    await asyncio.sleep(1.1)
    
    # Should succeed again
    assert await circuit_breaker.validate_transaction(tx)

async def test_profit_threshold(circuit_breaker):
    """Test minimum profit threshold."""
    # Transaction with sufficient profit
    tx1 = {
        'value': Web3.to_wei(1, 'ether'),
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert await circuit_breaker.validate_transaction(tx1)
    
    # Transaction with insufficient profit
    tx2 = {
        'value': Web3.to_wei(1, 'ether'),
        'expected_profit': Web3.to_wei(0.05, 'ether')
    }
    assert not await circuit_breaker.validate_transaction(tx2)

async def test_slippage_protection(circuit_breaker):
    """Test slippage protection."""
    # Transaction within slippage limit
    tx1 = {
        'value': Web3.to_wei(1, 'ether'),
        'expected_price': Web3.to_wei(100, 'ether'),
        'actual_price': Web3.to_wei(100.5, 'ether'),
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert await circuit_breaker.validate_transaction(tx1)
    
    # Transaction exceeding slippage limit
    tx2 = {
        'value': Web3.to_wei(1, 'ether'),
        'expected_price': Web3.to_wei(100, 'ether'),
        'actual_price': Web3.to_wei(102, 'ether'),
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert not await circuit_breaker.validate_transaction(tx2)

async def test_daily_loss_limit(circuit_breaker):
    """Test daily loss limit."""
    # Record losses up to limit
    await circuit_breaker.record_profit_loss(Decimal('-2.5'))
    assert not circuit_breaker.is_triggered()
    
    # Exceed daily loss limit
    await circuit_breaker.record_profit_loss(Decimal('-3'))
    assert circuit_breaker.is_triggered()

async def test_metrics_reset(circuit_breaker):
    """Test metrics reset functionality."""
    # Add some metrics
    tx = {
        'value': Web3.to_wei(10, 'ether'),
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert await circuit_breaker.validate_transaction(tx)
    await circuit_breaker.record_profit_loss(Decimal('-1'))
    
    # Wait for reset interval
    await asyncio.sleep(1.1)
    
    # Force a health check to process the reset
    await circuit_breaker._check_system_health()
    
    # Verify metrics were reset
    assert circuit_breaker.position_limit.current_size == Decimal('0')
    assert circuit_breaker.profit_metrics.current_daily_pnl == Decimal('0')

async def test_emergency_shutdown(circuit_breaker):
    """Test emergency shutdown."""
    assert not circuit_breaker.is_triggered()
    
    await circuit_breaker.emergency_shutdown()
    assert circuit_breaker.is_triggered()
    assert circuit_breaker.get_trigger_reason() == "Emergency shutdown activated"
    
    # Verify no transactions are accepted after shutdown
    tx = {
        'value': Web3.to_wei(1, 'ether'),
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert not await circuit_breaker.validate_transaction(tx)

async def test_breaker_reset(circuit_breaker):
    """Test circuit breaker reset functionality."""
    # Trigger the breaker
    await circuit_breaker.trigger_breaker("Test trigger")
    assert circuit_breaker.is_triggered()
    
    # Reset the breaker
    await circuit_breaker.reset_breaker()
    assert not circuit_breaker.is_triggered()
    assert circuit_breaker.get_trigger_reason() is None
    
    # Verify transactions are accepted after reset
    tx = {
        'value': Web3.to_wei(1, 'ether'),
        'expected_profit': Web3.to_wei(0.2, 'ether')
    }
    assert await circuit_breaker.validate_transaction(tx)
