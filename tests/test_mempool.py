"""Tests for mempool monitoring"""
import pytest
import pytest_asyncio
from mevbot.core.mempool import MempoolMonitor, Transaction
from eth_typing import Address, HexStr
import time
import asyncio
from web3 import Web3
from unittest.mock import Mock, AsyncMock

@pytest.fixture
def w3():
    # Create a mock Web3 instance
    mock_w3 = Mock(spec=Web3)
    mock_w3.eth = Mock()
    # Return empty list for pending transactions
    mock_w3.eth.get_pending_transactions = AsyncMock(return_value=[])
    mock_w3.eth.get_transaction = AsyncMock(return_value=None)
    return mock_w3

@pytest.fixture
def config():
    return {
        'max_pending_tx': 1000,
        'enable_custom_indexing': True,
        'cleanup_interval': 1  # Reduced for faster testing
    }

@pytest_asyncio.fixture
async def mempool_monitor(w3, config):
    """Create mempool monitor for testing"""
    monitor = MempoolMonitor(w3, config)
    await monitor.start()
    yield monitor
    await monitor.stop()

@pytest.mark.asyncio
async def test_transaction_tracking(mempool_monitor):
    """Test transaction tracking"""
    # Add test transaction
    tx_hash = HexStr('0x' + '1' * 64)
    tx = Transaction(
        hash=tx_hash,
        from_address=Address('0x' + '2' * 40),
        to_address=Address('0x' + '3' * 40),
        value=1000000000000000000,  # 1 ETH
        gas_price=20000000000,  # 20 GWEI
        gas_limit=21000,
        nonce=0,
        data=b'',
        timestamp=time.time()
    )
    
    mempool_monitor.transactions[tx_hash] = tx
    
    # Test retrieval
    result = await mempool_monitor.get_transaction(tx_hash)
    assert result is not None
    assert result.hash == tx_hash
    assert result.value == 1000000000000000000

@pytest.mark.asyncio
async def test_watch_address(mempool_monitor):
    """Test address watching"""
    test_address = Address('0x' + '1' * 40)
    
    # Add address to watch
    mempool_monitor.add_watch_address(test_address)
    assert test_address in mempool_monitor.watched_addresses
    
    # Remove address
    mempool_monitor.remove_watch_address(test_address)
    assert test_address not in mempool_monitor.watched_addresses

@pytest.mark.asyncio
async def test_cleanup(mempool_monitor):
    """Test old transaction cleanup"""
    # Add old transaction
    old_tx = Transaction(
        hash=HexStr('0x' + '1' * 64),
        from_address=Address('0x' + '2' * 40),
        to_address=Address('0x' + '3' * 40),
        value=1000000000000000000,
        gas_price=20000000000,
        gas_limit=21000,
        nonce=0,
        data=b'',
        timestamp=time.time() - 600  # 10 minutes old
    )
    mempool_monitor.transactions[old_tx.hash] = old_tx
    
    # Run cleanup
    await asyncio.sleep(2)  # Wait for monitor loop to run
    
    # Verify cleanup
    assert old_tx.hash not in mempool_monitor.transactions
