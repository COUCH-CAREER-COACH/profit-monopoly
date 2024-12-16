"""Tests for the SafetyCoordinator system."""
import os
import pytest
import asyncio
from unittest.mock import Mock, patch, AsyncMock
from web3 import Web3
from decimal import Decimal

from mevbot.core.safety.coordinator import SafetyCoordinator
from mevbot.core.safety.emergency_manager import EmergencyLevel

@pytest.fixture
def web3_mock():
    """Create a mock Web3 instance."""
    mock = Mock()
    mock.eth = Mock()
    mock.eth.gas_price = 50000000000  # 50 gwei
    mock.eth.get_block = AsyncMock()
    mock.from_wei = Web3.from_wei
    mock.to_wei = Web3.to_wei
    return mock

@pytest.fixture
def test_config():
    """Create test configuration."""
    return {
        'key_manager': {
            'keystore_path': '/tmp/test_keystore'
        },
        'circuit_breaker': {
            'max_position_size': 10.0,
            'max_gas_price_gwei': 100,
            'max_daily_gas_spend': 1.0
        },
        'emergency_manager': {
            'notifications': {'enabled': True},
            'emergency_contacts': ['test@example.com']
        },
        'max_gas_price_gwei': 100,
        'contract_whitelist': [
            '0x' + 'a' * 40,
            '0x' + 'b' * 40
        ],
        'average_transaction_value': 0.1
    }

@pytest.fixture
def safety_coordinator(test_config, web3_mock):
    """Create SafetyCoordinator instance for testing."""
    return SafetyCoordinator(test_config, web3_mock)

@pytest.mark.asyncio
async def test_start_stop(safety_coordinator):
    """Test starting and stopping the coordinator."""
    await safety_coordinator.start()
    assert safety_coordinator._monitoring_task is not None
    assert safety_coordinator._metrics_task is not None
    
    await safety_coordinator.stop()
    assert safety_coordinator._monitoring_task.cancelled()
    assert safety_coordinator._metrics_task.cancelled()

@pytest.mark.asyncio
async def test_validate_transaction(safety_coordinator):
    """Test transaction validation."""
    # Valid transaction
    tx = {
        'to': safety_coordinator.config['contract_whitelist'][0],
        'value': Web3.to_wei(1, 'ether'),
        'gasPrice': Web3.to_wei(50, 'gwei')
    }
    assert await safety_coordinator.validate_transaction(tx)
    
    # Invalid gas price
    tx['gasPrice'] = Web3.to_wei(150, 'gwei')
    assert not await safety_coordinator.validate_transaction(tx)
    
    # Non-whitelisted contract
    tx['to'] = '0x' + 'f' * 40
    assert not await safety_coordinator.validate_transaction(tx)

@pytest.mark.asyncio
async def test_strategy_registration(safety_coordinator):
    """Test strategy registration and unregistration."""
    strategy_id = "test_strategy"
    
    await safety_coordinator.register_strategy(strategy_id)
    assert strategy_id in safety_coordinator.active_strategies
    
    await safety_coordinator.unregister_strategy(strategy_id)
    assert strategy_id not in safety_coordinator.active_strategies

@pytest.mark.asyncio
async def test_incident_recording(safety_coordinator):
    """Test incident recording and risk level updates."""
    with patch.object(safety_coordinator.emergency_manager, 'trigger_shutdown') as mock_shutdown:
        # Record INFO incident
        await safety_coordinator.record_incident(
            EmergencyLevel.INFO,
            "Test incident",
            "Test"
        )
        assert safety_coordinator.metrics.emergency_events == 1
        assert safety_coordinator.metrics.current_risk_level == "LOW"
        assert not mock_shutdown.called
        
        # Record CRITICAL incident
        await safety_coordinator.record_incident(
            EmergencyLevel.CRITICAL,
            "Critical incident",
            "Test"
        )
        assert safety_coordinator.metrics.emergency_events == 2
        assert safety_coordinator.metrics.current_risk_level == "HIGH"
        assert mock_shutdown.called

@pytest.mark.asyncio
async def test_system_resource_monitoring(safety_coordinator):
    """Test system resource monitoring."""
    with patch('psutil.cpu_percent', return_value=95):
        assert not safety_coordinator._check_system_resources()
    
    with patch('psutil.cpu_percent', return_value=50):
        with patch('psutil.virtual_memory') as mock_memory:
            mock_memory.return_value.percent = 95
            assert not safety_coordinator._check_system_resources()
    
    with patch('psutil.cpu_percent', return_value=50):
        with patch('psutil.virtual_memory') as mock_memory:
            mock_memory.return_value.percent = 50
            with patch('psutil.disk_usage') as mock_disk:
                mock_disk.return_value.percent = 95
                assert not safety_coordinator._check_system_resources()
                
                mock_disk.return_value.percent = 50
                assert safety_coordinator._check_system_resources()

@pytest.mark.asyncio
async def test_network_conditions(safety_coordinator, web3_mock):
    """Test network condition monitoring."""
    # Test high gas price
    web3_mock.eth.gas_price = Web3.to_wei(150, 'gwei')
    assert not await safety_coordinator._check_network_conditions()
    
    # Test normal conditions
    web3_mock.eth.gas_price = Web3.to_wei(50, 'gwei')
    web3_mock.eth.get_block.return_value = {
        'timestamp': int(time.time()) - 10
    }
    assert await safety_coordinator._check_network_conditions()
    
    # Test network congestion
    web3_mock.eth.get_block.return_value = {
        'timestamp': int(time.time()) - 100
    }
    assert not await safety_coordinator._check_network_conditions()

@pytest.mark.asyncio
async def test_metrics_update(safety_coordinator):
    """Test metrics updating."""
    safety_coordinator.metrics.circuit_breaks = 5
    
    # Run metrics update
    await safety_coordinator._update_metrics()
    
    expected_prevented_loss = 5 * float(
        safety_coordinator.config['average_transaction_value']
    )
    assert safety_coordinator.metrics.total_prevented_loss == expected_prevented_loss

@pytest.mark.asyncio
async def test_emergency_system_initialization(safety_coordinator):
    """Test emergency system initialization."""
    with patch.object(safety_coordinator.emergency_manager, 'recovery_mode', True):
        with patch.object(safety_coordinator, '_handle_recovery') as mock_recovery:
            await safety_coordinator._initialize_emergency_system()
            mock_recovery.assert_called_once()

@pytest.mark.asyncio
async def test_coordinated_shutdown(safety_coordinator):
    """Test coordinated shutdown process."""
    with patch.object(safety_coordinator.emergency_manager, 'trigger_shutdown') as mock_shutdown:
        await safety_coordinator.record_incident(
            EmergencyLevel.FATAL,
            "Fatal error",
            "Test"
        )
        mock_shutdown.assert_called_once_with("Fatal error", EmergencyLevel.FATAL)
        
        # Verify metrics
        assert safety_coordinator.metrics.emergency_events == 1
        assert safety_coordinator.metrics.current_risk_level == "HIGH"
