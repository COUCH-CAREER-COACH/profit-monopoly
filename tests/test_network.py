"""Tests for network optimization"""
import pytest
from unittest.mock import Mock, patch, MagicMock
from mevbot.core.network import DPDKManager, DPDKConfig

@pytest.fixture
def config():
    """Test configuration"""
    return {
        'dpdk_cores': [0, 1],
        'dpdk_memory_channels': 4,
        'dpdk_huge_pages': 1024,
        'dpdk_port_mask': 0x1,
        'dpdk_rx_queues': 4,
        'dpdk_tx_queues': 4
    }

def test_dpdk_config(config):
    """Test DPDK configuration"""
    dpdk_config = DPDKConfig(
        cores=config['dpdk_cores'],
        memory_channels=config['dpdk_memory_channels'],
        huge_pages=config['dpdk_huge_pages'],
        port_mask=config['dpdk_port_mask'],
        rx_queues=config['dpdk_rx_queues'],
        tx_queues=config['dpdk_tx_queues']
    )
    assert dpdk_config.cores == [0, 1]
    assert dpdk_config.memory_channels == 4
    assert dpdk_config.huge_pages == 1024
    assert dpdk_config.port_mask == 0x1
    assert dpdk_config.rx_queues == 4
    assert dpdk_config.tx_queues == 4

@pytest.mark.skipif(not pytest.DPDK_AVAILABLE, reason="DPDK not available")
def test_dpdk_initialization(config):
    """Test DPDK initialization"""
    manager = DPDKManager(config)
    assert manager.initialize() is True
    assert manager.initialized is True

@pytest.mark.skipif(not pytest.DPDK_AVAILABLE, reason="DPDK not available")
def test_dpdk_mempools(config):
    """Test DPDK memory pools"""
    manager = DPDKManager(config)
    manager.initialize()
    assert manager.rx_mempool is not None
    assert manager.tx_mempool is not None

@pytest.mark.skipif(not pytest.DPDK_AVAILABLE, reason="DPDK not available")
def test_dpdk_ports(config):
    """Test DPDK port configuration"""
    manager = DPDKManager(config)
    manager.initialize()
    # Verify port initialization by checking if at least one port is available
    assert manager.dpdk.rte_eth_dev_count_avail() > 0

# Mock tests that can run without DPDK
@pytest.mark.parametrize("mock_available", [True, False])
def test_dpdk_initialization_mock(config, mock_available):
    """Test DPDK initialization with mocked library"""
    with patch('ctypes.CDLL') as mock_cdll:
        # Create mock DPDK library
        mock_dpdk = MagicMock()
        mock_dpdk.rte_eal_init.return_value = 0 if mock_available else -1
        mock_dpdk.rte_pktmbuf_pool_create.return_value = 1 if mock_available else 0
        mock_dpdk.rte_eth_dev_count_avail.return_value = 1 if mock_available else 0
        mock_dpdk.rte_eth_dev_configure.return_value = 0 if mock_available else -1
        mock_dpdk.rte_eth_rx_queue_setup.return_value = 0 if mock_available else -1
        mock_dpdk.rte_eth_tx_queue_setup.return_value = 0 if mock_available else -1
        mock_dpdk.rte_eth_dev_start.return_value = 0 if mock_available else -1
        mock_cdll.return_value = mock_dpdk
        
        manager = DPDKManager(config)
        result = manager.initialize()
        
        assert result == mock_available
        assert manager.initialized == mock_available
        mock_cdll.assert_called_once_with('librte.so')

def test_dpdk_mempool_creation_mock(config):
    """Test mempool creation with mocked DPDK"""
    with patch('ctypes.CDLL') as mock_cdll:
        # Setup mock DPDK library
        mock_dpdk = MagicMock()
        mock_dpdk.rte_eal_init.return_value = 0
        mock_dpdk.rte_pktmbuf_pool_create.side_effect = [1, 1]  # RX and TX pools
        mock_dpdk.rte_eth_dev_count_avail.return_value = 1
        mock_dpdk.rte_eth_dev_configure.return_value = 0
        mock_dpdk.rte_eth_rx_queue_setup.return_value = 0
        mock_dpdk.rte_eth_tx_queue_setup.return_value = 0
        mock_dpdk.rte_eth_dev_start.return_value = 0
        mock_cdll.return_value = mock_dpdk
        
        manager = DPDKManager(config)
        manager.initialize()
        
        # Verify mempool creation calls
        assert mock_dpdk.rte_pktmbuf_pool_create.call_count == 2
        rx_call, tx_call = mock_dpdk.rte_pktmbuf_pool_create.call_args_list
        assert b"rx_mempool" in rx_call[0]
        assert b"tx_mempool" in tx_call[0]

def test_dpdk_port_configuration_mock(config):
    """Test port configuration with mocked DPDK"""
    with patch('ctypes.CDLL') as mock_cdll:
        # Setup mock DPDK library
        mock_dpdk = MagicMock()
        mock_dpdk.rte_eal_init.return_value = 0
        mock_dpdk.rte_eth_dev_count_avail.return_value = 1
        mock_dpdk.rte_eth_dev_configure.return_value = 0
        mock_dpdk.rte_eth_rx_queue_setup.return_value = 0
        mock_dpdk.rte_eth_tx_queue_setup.return_value = 0
        mock_dpdk.rte_eth_dev_start.return_value = 0
        mock_cdll.return_value = mock_dpdk
        
        manager = DPDKManager(config)
        manager.initialize()
        
        # Verify port configuration sequence
        mock_dpdk.rte_eth_dev_configure.assert_called()
        mock_dpdk.rte_eth_rx_queue_setup.assert_called()
        mock_dpdk.rte_eth_tx_queue_setup.assert_called()
        mock_dpdk.rte_eth_dev_start.assert_called()

def test_dpdk_error_handling_mock(config):
    """Test error handling with mocked DPDK"""
    with patch('ctypes.CDLL') as mock_cdll:
        # Setup mock DPDK library with errors
        mock_dpdk = MagicMock()
        mock_dpdk.rte_eal_init.return_value = -1  # Simulate EAL init failure
        mock_cdll.return_value = mock_dpdk
        
        manager = DPDKManager(config)
        result = manager.initialize()
        
        assert result is False
        assert not manager.initialized
        mock_dpdk.rte_pktmbuf_pool_create.assert_not_called()  # Should not proceed to mempool creation
