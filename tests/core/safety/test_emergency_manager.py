"""Tests for the EmergencyManager system."""
import os
import json
import pytest
import asyncio
from unittest.mock import Mock, patch, AsyncMock
from web3 import Web3
from eth_account import Account

from mevbot.core.safety.emergency_manager import (
    EmergencyManager,
    EmergencyLevel,
    EmergencyEvent
)

@pytest.fixture
def web3_mock():
    """Create a mock Web3 instance."""
    mock = Mock()
    mock.eth = Mock()
    mock.eth.get_transaction = AsyncMock()
    mock.eth.get_transaction_receipt = AsyncMock()
    mock.eth.send_raw_transaction = AsyncMock()
    return mock

@pytest.fixture
def test_config():
    """Create test configuration."""
    return {
        'notifications': {
            'enabled': True,
            'min_level': 'WARNING'
        },
        'emergency_contacts': ['test@example.com'],
        'discord_webhook': 'https://discord.webhook/test',
        'telegram_token': 'test_token',
        'telegram_chat_ids': ['123456789'],
        'email': {
            'smtp_server': 'smtp.test.com',
            'smtp_port': 587,
            'username': 'test@test.com',
            'password': 'test_password'
        },
        'private_key': '0x' + '1' * 64
    }

@pytest.fixture
def emergency_manager(test_config, web3_mock, tmp_path):
    """Create EmergencyManager instance for testing."""
    manager = EmergencyManager(test_config, web3_mock)
    manager.state_file = str(tmp_path / 'emergency_state.json')
    return manager

@pytest.mark.asyncio
async def test_trigger_shutdown(emergency_manager):
    """Test emergency shutdown triggering."""
    with patch.object(emergency_manager, '_notify_emergency_contacts') as mock_notify:
        with patch.object(emergency_manager, '_unwind_positions') as mock_unwind:
            with patch.object(emergency_manager, '_cancel_pending_transactions') as mock_cancel:
                await emergency_manager.trigger_shutdown(
                    "Test emergency",
                    EmergencyLevel.CRITICAL
                )
                
                assert emergency_manager.shutdown_triggered
                mock_notify.assert_called_once()
                mock_unwind.assert_called_once()
                mock_cancel.assert_called_once()

@pytest.mark.asyncio
async def test_cancel_pending_transactions(emergency_manager, web3_mock):
    """Test transaction cancellation."""
    # Add pending transaction
    tx_hash = '0x' + '1' * 64
    emergency_manager.pending_transactions.add(tx_hash)
    
    # Mock transaction data
    mock_tx = {
        'from': '0x' + 'a' * 40,
        'nonce': 1,
        'gasPrice': 1000000000
    }
    web3_mock.eth.get_transaction.return_value = mock_tx
    
    await emergency_manager._cancel_pending_transactions()
    
    # Verify cancellation transaction was sent
    assert web3_mock.eth.send_raw_transaction.called
    assert tx_hash in emergency_manager.cancelled_transactions
    assert tx_hash not in emergency_manager.pending_transactions

@pytest.mark.asyncio
async def test_preserve_and_load_state(emergency_manager):
    """Test system state preservation and loading."""
    # Add some test data
    emergency_manager.pending_transactions.add('0x' + '1' * 64)
    emergency_manager.cancelled_transactions.add('0x' + '2' * 64)
    emergency_manager.shutdown_triggered = True
    
    # Save state
    await emergency_manager._preserve_system_state()
    
    # Create new instance and load state
    new_manager = EmergencyManager(emergency_manager.config, emergency_manager.web3)
    new_manager.state_file = emergency_manager.state_file
    new_manager._load_state()
    
    assert new_manager.shutdown_triggered
    assert new_manager.recovery_mode
    assert len(new_manager.pending_transactions) == 1
    assert len(new_manager.cancelled_transactions) == 1

@pytest.mark.asyncio
async def test_email_notification(emergency_manager):
    """Test email notification system."""
    with patch('smtplib.SMTP') as mock_smtp:
        mock_server = Mock()
        mock_smtp.return_value.__enter__.return_value = mock_server
        
        event = EmergencyEvent(
            level=EmergencyLevel.CRITICAL,
            message="Test emergency",
            timestamp=1234567890,
            source="Test"
        )
        
        await emergency_manager._send_email_alert(event)
        
        assert mock_server.send_message.called
        assert mock_server.login.called

@pytest.mark.asyncio
async def test_discord_notification(emergency_manager):
    """Test Discord notification system."""
    with patch('aiohttp.ClientSession.post') as mock_post:
        mock_response = AsyncMock()
        mock_response.status = 204
        mock_post.return_value.__aenter__.return_value = mock_response
        
        event = EmergencyEvent(
            level=EmergencyLevel.CRITICAL,
            message="Test emergency",
            timestamp=1234567890,
            source="Test"
        )
        
        await emergency_manager._send_discord_alert(event)
        assert mock_post.called

@pytest.mark.asyncio
async def test_telegram_notification(emergency_manager):
    """Test Telegram notification system."""
    with patch('aiohttp.ClientSession.get') as mock_get:
        mock_response = AsyncMock()
        mock_response.status = 200
        mock_get.return_value.__aenter__.return_value = mock_response
        
        event = EmergencyEvent(
            level=EmergencyLevel.CRITICAL,
            message="Test emergency",
            timestamp=1234567890,
            source="Test"
        )
        
        await emergency_manager._send_telegram_alert(event)
        assert mock_get.called

@pytest.mark.asyncio
async def test_emergency_contacts_management(emergency_manager):
    """Test emergency contacts management."""
    new_contact = "new@example.com"
    emergency_manager.add_emergency_contact(new_contact)
    assert new_contact in emergency_manager.emergency_contacts
    
    emergency_manager.remove_emergency_contact(new_contact)
    assert new_contact not in emergency_manager.emergency_contacts

@pytest.mark.asyncio
async def test_background_tasks(emergency_manager):
    """Test background tasks initialization and cleanup."""
    assert len(emergency_manager._background_tasks) > 0
    
    # Test cleanup
    await emergency_manager.cleanup()
    assert len(emergency_manager._background_tasks) == 0
    
    # Verify all tasks are cancelled
    for task in emergency_manager._background_tasks:
        assert task.cancelled()

@pytest.mark.asyncio
async def test_fatal_emergency_level(emergency_manager):
    """Test FATAL emergency level handling."""
    with patch('os._exit') as mock_exit:
        with patch.object(emergency_manager, '_notify_emergency_contacts'):
            with patch.object(emergency_manager, '_unwind_positions'):
                with patch.object(emergency_manager, '_cancel_pending_transactions'):
                    await emergency_manager.trigger_shutdown(
                        "Fatal error",
                        EmergencyLevel.FATAL
                    )
                    mock_exit.assert_called_once_with(1)

@pytest.mark.asyncio
async def test_transaction_monitoring(emergency_manager, web3_mock):
    """Test transaction monitoring system."""
    tx_hash = '0x' + '1' * 64
    emergency_manager.pending_transactions.add(tx_hash)
    
    # Mock successful transaction
    web3_mock.eth.get_transaction_receipt.return_value = {'status': 1}
    
    # Run monitoring for a short period
    monitoring_task = asyncio.create_task(emergency_manager._monitor_pending_transactions())
    await asyncio.sleep(0.1)
    monitoring_task.cancel()
    
    try:
        await monitoring_task
    except asyncio.CancelledError:
        pass
    
    assert tx_hash not in emergency_manager.pending_transactions
