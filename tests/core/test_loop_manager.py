"""Tests for the LoopManager."""
import pytest
import asyncio
from unittest.mock import Mock, patch, AsyncMock
from web3 import Web3

from mevbot.core.loop_manager import LoopManager
from mevbot.core.strategy_base import BaseStrategy
from mevbot.core.safety.emergency_manager import EmergencyLevel

class MockStrategy(BaseStrategy):
    """Mock strategy for testing."""
    def __init__(self, strategy_id: str):
        self.strategy_id = strategy_id
        self.ready = True
        self.stopped = False
    
    def get_id(self) -> str:
        return self.strategy_id
    
    def is_ready(self) -> bool:
        return self.ready
    
    async def execute(self):
        return {'value': Web3.to_wei(1, 'ether')}
    
    async def stop(self):
        self.stopped = True

@pytest.fixture
def web3_mock():
    """Create a mock Web3 instance."""
    mock = Mock()
    mock.eth = Mock()
    mock.eth.send_raw_transaction = AsyncMock()
    mock.eth.account = Mock()
    mock.eth.account.sign_transaction = Mock()
    return mock

@pytest.fixture
def test_config():
    """Create test configuration."""
    return {
        'safety': {
            'key_manager': {'keystore_path': '/tmp/test_keystore'},
            'circuit_breaker': {'max_position_size': 10.0},
            'emergency_manager': {'notifications': {'enabled': True}}
        },
        'key_id': 'test_key',
        'key_password': 'test_password'
    }

@pytest.fixture
def loop_manager(test_config, web3_mock):
    """Create LoopManager instance for testing."""
    return LoopManager(test_config, web3_mock)

@pytest.mark.asyncio
async def test_start_stop(loop_manager):
    """Test starting and stopping the loop manager."""
    # Start loop manager
    start_task = asyncio.create_task(loop_manager.start())
    await asyncio.sleep(0.1)  # Let it run briefly
    
    assert loop_manager.running
    assert loop_manager.start_time is not None
    
    # Stop loop manager
    await loop_manager.stop()
    await start_task
    
    assert not loop_manager.running
    assert all(strategy.stopped for strategy in loop_manager.strategies.values())

@pytest.mark.asyncio
async def test_strategy_management(loop_manager):
    """Test adding and removing strategies."""
    strategy = MockStrategy("test_strategy")
    
    # Add strategy
    loop_manager.add_strategy(strategy)
    assert "test_strategy" in loop_manager.strategies
    
    # Try adding duplicate
    with pytest.raises(ValueError):
        loop_manager.add_strategy(strategy)
    
    # Remove strategy
    loop_manager.remove_strategy("test_strategy")
    assert "test_strategy" not in loop_manager.strategies
    
    # Try removing non-existent
    with pytest.raises(ValueError):
        loop_manager.remove_strategy("non_existent")

@pytest.mark.asyncio
async def test_strategy_execution(loop_manager):
    """Test safe strategy execution."""
    strategy = MockStrategy("test_strategy")
    loop_manager.add_strategy(strategy)
    
    # Mock safety validations
    loop_manager.safety.validate_transaction = AsyncMock(return_value=True)
    loop_manager.safety._check_network_conditions = AsyncMock(return_value=True)
    loop_manager.safety._check_system_resources = Mock(return_value=True)
    
    # Execute strategy
    await loop_manager._execute_strategy_safely(strategy)
    
    assert loop_manager.total_executions == 1
    assert loop_manager.successful_executions == 1

@pytest.mark.asyncio
async def test_transaction_submission(loop_manager, web3_mock):
    """Test transaction submission process."""
    tx = {'value': Web3.to_wei(1, 'ether')}
    
    # Mock safety validation
    loop_manager.safety.validate_transaction = AsyncMock(return_value=True)
    
    # Mock key manager
    loop_manager.safety.key_manager.get_private_key = AsyncMock(
        return_value="test_private_key"
    )
    
    # Mock transaction signing
    signed_tx = Mock()
    signed_tx.rawTransaction = b'signed_tx_data'
    web3_mock.eth.account.sign_transaction.return_value = signed_tx
    
    # Mock transaction sending
    tx_hash = '0x' + '1' * 64
    web3_mock.eth.send_raw_transaction.return_value = tx_hash
    
    # Submit transaction
    result = await loop_manager._submit_transaction(tx)
    
    assert result == tx_hash
    assert tx_hash in loop_manager.pending_transactions

@pytest.mark.asyncio
async def test_safety_integration(loop_manager):
    """Test safety system integration."""
    strategy = MockStrategy("test_strategy")
    loop_manager.add_strategy(strategy)
    
    # Mock strategy to raise an error
    strategy.execute = AsyncMock(side_effect=Exception("Test error"))
    
    # Mock safety system
    loop_manager.safety.record_incident = AsyncMock()
    
    # Execute strategy
    await loop_manager._execute_strategy_safely(strategy)
    
    # Verify incident was recorded
    loop_manager.safety.record_incident.assert_called_once_with(
        EmergencyLevel.WARNING,
        "Strategy execution error: Test error",
        "test_strategy"
    )

@pytest.mark.asyncio
async def test_performance_metrics(loop_manager):
    """Test performance metrics logging."""
    strategy = MockStrategy("test_strategy")
    loop_manager.add_strategy(strategy)
    
    # Mock successful execution
    loop_manager.safety.validate_transaction = AsyncMock(return_value=True)
    loop_manager.safety._check_network_conditions = AsyncMock(return_value=True)
    loop_manager.safety._check_system_resources = Mock(return_value=True)
    
    # Start loop manager
    loop_manager.start_time = time.time()
    
    # Execute strategy multiple times
    for _ in range(5):
        await loop_manager._execute_strategy_safely(strategy)
    
    # Log metrics
    with patch('logging.Logger.info') as mock_log:
        loop_manager._log_performance_metrics()
        mock_log.assert_called_once()
        
        # Verify metrics
        log_message = mock_log.call_args[0][0]
        assert "Total Executions: 5" in log_message
        assert "Successful Executions: 5" in log_message
        assert "Success Rate: 100.00%" in log_message

@pytest.mark.asyncio
async def test_error_handling(loop_manager):
    """Test error handling in main loop."""
    strategy = MockStrategy("test_strategy")
    strategy.execute = AsyncMock(side_effect=Exception("Test error"))
    loop_manager.add_strategy(strategy)
    
    # Mock safety system
    loop_manager.safety.record_incident = AsyncMock()
    
    # Run loop briefly
    loop_manager.running = True
    loop_task = asyncio.create_task(loop_manager._run_loop())
    await asyncio.sleep(0.1)
    loop_manager.running = False
    await loop_task
    
    # Verify incident was recorded
    assert loop_manager.safety.record_incident.called

@pytest.mark.asyncio
async def test_strategy_execution_conditions(loop_manager):
    """Test conditions for strategy execution."""
    strategy = MockStrategy("test_strategy")
    
    # Test when strategy is not ready
    strategy.ready = False
    assert not await loop_manager._should_execute_strategy(strategy)
    
    # Test when network conditions are bad
    strategy.ready = True
    loop_manager.safety._check_network_conditions = AsyncMock(return_value=False)
    assert not await loop_manager._should_execute_strategy(strategy)
    
    # Test when system resources are low
    loop_manager.safety._check_network_conditions = AsyncMock(return_value=True)
    loop_manager.safety._check_system_resources = Mock(return_value=False)
    assert not await loop_manager._should_execute_strategy(strategy)
    
    # Test when all conditions are good
    loop_manager.safety._check_system_resources = Mock(return_value=True)
    assert await loop_manager._should_execute_strategy(strategy)
