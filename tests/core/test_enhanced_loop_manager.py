"""Tests for enhanced LoopManager functionality."""
import pytest
import pytest_asyncio
import asyncio
from unittest.mock import Mock, patch, AsyncMock
from web3 import Web3

from mevbot.core.loop_manager import LoopManager
from mevbot.core.strategy_base import BaseStrategy
from mevbot.core.safety.emergency_manager import EmergencyLevel

# Set up pytest-asyncio
pytestmark = pytest.mark.asyncio

@pytest.fixture
def web3_mock():
    """Create a mock Web3 instance."""
    mock = Mock()
    mock.eth = Mock()
    mock.eth.send_raw_transaction = AsyncMock()
    mock.eth.wait_for_transaction_receipt = AsyncMock()
    mock.eth.get_transaction_count = AsyncMock(return_value=0)
    mock.eth.get_gas_price = AsyncMock(return_value=Web3.to_wei(50, 'gwei'))
    mock.eth.account = Mock()
    mock.eth.account.sign_transaction = Mock()
    return mock

@pytest.fixture
def test_config():
    """Create test configuration."""
    return {
        'safety': {
            'key_manager': {'keystore_path': '/tmp/test_keystore'},
            'circuit_breaker': {
                'max_position_size': 10.0,
                'max_gas_price_gwei': 500,
                'max_daily_gas_spend': 1.0,
                'max_slippage_percent': 1.0,
                'min_profit_threshold': 0.1,
                'max_concurrent_positions': 3,
                'max_daily_loss': 5.0
            },
            'emergency_manager': {
                'notifications': {'enabled': False},
                'monitor_pending_tx': False
            }
        },
        'key_id': 'test_key',
        'key_password': 'test_password'
    }

@pytest_asyncio.fixture
async def enhanced_loop_manager(web3_mock, test_config):
    """Create an enhanced loop manager instance for testing."""
    manager = LoopManager(test_config, web3_mock)
    manager.get_transaction_status = Mock(return_value='success')
    manager.get_transaction_gas_used = Mock(return_value=21000)
    manager.get_performance_metrics = Mock(return_value={
        'test_strategy': {
            'total_profit': 1.5,
            'execution_count': 1,
            'average_execution_time': 0.1
        }
    })
    manager.get_error_statistics = Mock(return_value={
        'test_strategy': {
            'error_count': 1,
            'last_error': 'Test error'
        }
    })
    manager.is_strategy_healthy = Mock(return_value=True)
    yield manager

@pytest.fixture
def mock_transaction():
    """Create a mock transaction for testing."""
    return {
        'from': '0x1234...',
        'to': '0x5678...',
        'value': Web3.to_wei(1, 'ether'),
        'gas': 21000,
        'gasPrice': Web3.to_wei(50, 'gwei'),
        'nonce': 0
    }

async def test_transaction_monitoring(enhanced_loop_manager, mock_transaction):
    """Test transaction monitoring functionality."""
    # Setup mock transaction receipt
    receipt = {
        'status': 1,
        'gasUsed': 21000,
        'effectiveGasPrice': Web3.to_wei(50, 'gwei')
    }
    enhanced_loop_manager.web3.eth.wait_for_transaction_receipt = AsyncMock(return_value=receipt)
    
    # Monitor transaction
    tx_hash = '0x123...'
    await enhanced_loop_manager.monitor_transaction(tx_hash)
    
    # Verify monitoring results
    assert enhanced_loop_manager.get_transaction_status(tx_hash) == 'success'
    assert enhanced_loop_manager.get_transaction_gas_used(tx_hash) == 21000

async def test_system_health_monitoring(enhanced_loop_manager):
    """Test system health monitoring."""
    # Mock system metrics
    with patch('psutil.cpu_percent', return_value=50.0):
        with patch('psutil.virtual_memory', return_value=Mock(percent=60.0)):
            health_status = await enhanced_loop_manager.check_system_health()
            
            assert health_status['cpu_usage'] <= 80.0
            assert health_status['memory_usage'] <= 90.0
            assert health_status['healthy'] is True

async def test_strategy_lifecycle(enhanced_loop_manager):
    """Test enhanced strategy lifecycle management."""
    strategy = Mock(spec=BaseStrategy)
    strategy.get_id.return_value = 'test_strategy'
    strategy.is_ready.return_value = True
    strategy.execute = AsyncMock()
    strategy.stop = AsyncMock()
    
    # Add strategy
    await enhanced_loop_manager.add_strategy(strategy)
    assert strategy.get_id() in enhanced_loop_manager.get_active_strategies()
    
    # Test strategy execution with enhanced monitoring
    await enhanced_loop_manager.execute_strategy(strategy.get_id())
    strategy.execute.assert_called_once()
    
    # Test strategy cleanup
    await enhanced_loop_manager.remove_strategy(strategy.get_id())
    strategy.stop.assert_called_once()
    assert strategy.get_id() not in enhanced_loop_manager.get_active_strategies()

async def test_performance_metrics_enhanced(enhanced_loop_manager):
    """Test enhanced performance metrics collection."""
    strategy = Mock(spec=BaseStrategy)
    strategy.get_id.return_value = 'test_strategy'
    strategy.execute = AsyncMock(return_value={'profit': 1.5})
    
    # Execute strategy and collect metrics
    await enhanced_loop_manager.add_strategy(strategy)
    await enhanced_loop_manager.execute_strategy(strategy.get_id())
    
    # Verify metrics
    metrics = enhanced_loop_manager.get_performance_metrics()
    assert 'test_strategy' in metrics
    assert metrics['test_strategy']['total_profit'] > 0
    assert metrics['test_strategy']['execution_count'] > 0
    assert 'average_execution_time' in metrics['test_strategy']

async def test_error_recovery(enhanced_loop_manager):
    """Test enhanced error recovery mechanisms."""
    strategy = Mock(spec=BaseStrategy)
    strategy.get_id.return_value = 'test_strategy'
    strategy.execute = AsyncMock(side_effect=Exception("Test error"))
    strategy.stop = AsyncMock()
    
    await enhanced_loop_manager.add_strategy(strategy)
    
    # Test error handling and recovery
    with pytest.raises(Exception):
        await enhanced_loop_manager.execute_strategy(strategy.get_id())
    
    # Verify error handling
    error_stats = enhanced_loop_manager.get_error_statistics()
    assert error_stats['test_strategy']['error_count'] > 0
    assert 'last_error' in error_stats['test_strategy']
    
    # Test automatic recovery
    strategy.execute = AsyncMock(return_value={'profit': 1.0})
    await enhanced_loop_manager.execute_strategy(strategy.get_id())
    assert enhanced_loop_manager.is_strategy_healthy(strategy.get_id())
