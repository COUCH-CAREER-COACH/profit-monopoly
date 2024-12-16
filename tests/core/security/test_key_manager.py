"""Tests for the SecureKeyManager."""
import os
import pytest
import tempfile
from pathlib import Path
from eth_account import Account

from mevbot.core.security.key_manager import SecureKeyManager

@pytest.fixture
def test_config():
    """Create a temporary directory for testing."""
    with tempfile.TemporaryDirectory() as tmpdir:
        yield {'keystore_path': tmpdir}

@pytest.fixture
def key_manager(test_config):
    """Create a SecureKeyManager instance for testing."""
    return SecureKeyManager(test_config)

@pytest.fixture
def test_key_data():
    """Generate test key data."""
    account = Account.create()
    return {
        'key_id': 'test_key',
        'private_key': account.key.hex(),
        'address': account.address,
        'password': 'test_password'
    }

def test_initialize_keystore(key_manager, test_config):
    """Test keystore initialization."""
    keystore_path = Path(test_config['keystore_path'])
    master_key_path = keystore_path / '.master.key'
    keys_file = keystore_path / 'encrypted_keys.json'
    
    assert keystore_path.exists()
    assert master_key_path.exists()
    assert keys_file.exists()
    
    # Check permissions
    assert oct(os.stat(keystore_path).st_mode)[-3:] == '700'
    assert oct(os.stat(master_key_path).st_mode)[-3:] == '600'
    assert oct(os.stat(keys_file).st_mode)[-3:] == '600'

def test_add_private_key(key_manager, test_key_data):
    """Test adding a private key."""
    success = key_manager.add_private_key(
        test_key_data['key_id'],
        test_key_data['private_key'],
        test_key_data['password']
    )
    assert success
    
    # Verify key is listed
    keys = key_manager.list_keys()
    assert test_key_data['key_id'] in keys
    assert keys[test_key_data['key_id']] == test_key_data['address']

def test_get_private_key(key_manager, test_key_data):
    """Test retrieving a private key."""
    # Add key first
    key_manager.add_private_key(
        test_key_data['key_id'],
        test_key_data['private_key'],
        test_key_data['password']
    )
    
    # Retrieve key
    retrieved_key = key_manager.get_private_key(
        test_key_data['key_id'],
        test_key_data['password']
    )
    
    assert retrieved_key == test_key_data['private_key']
    
    # Try with wrong password
    wrong_key = key_manager.get_private_key(
        test_key_data['key_id'],
        'wrong_password'
    )
    assert wrong_key is None

def test_remove_key(key_manager, test_key_data):
    """Test removing a private key."""
    # Add key first
    key_manager.add_private_key(
        test_key_data['key_id'],
        test_key_data['private_key'],
        test_key_data['password']
    )
    
    # Remove key
    success = key_manager.remove_key(
        test_key_data['key_id'],
        test_key_data['password']
    )
    assert success
    
    # Verify key is removed
    keys = key_manager.list_keys()
    assert test_key_data['key_id'] not in keys
    
    # Try to remove with wrong password
    success = key_manager.remove_key(
        test_key_data['key_id'],
        'wrong_password'
    )
    assert not success

def test_invalid_private_key(key_manager):
    """Test handling of invalid private keys."""
    invalid_keys = [
        'not_a_key',
        '0x123',  # Too short
        'a' * 65,  # Too long
        '0x' + 'g' * 64  # Invalid hex
    ]
    
    for invalid_key in invalid_keys:
        success = key_manager.add_private_key(
            'test_invalid',
            invalid_key,
            'test_password'
        )
        assert not success

def test_list_keys(key_manager, test_key_data):
    """Test listing stored keys."""
    # Initially empty
    assert len(key_manager.list_keys()) == 0
    
    # Add multiple keys
    accounts = [Account.create() for _ in range(3)]
    for i, account in enumerate(accounts):
        key_manager.add_private_key(
            f'key_{i}',
            account.key.hex(),
            test_key_data['password']
        )
    
    keys = key_manager.list_keys()
    assert len(keys) == 3
    for i, account in enumerate(accounts):
        assert keys[f'key_{i}'] == account.address
