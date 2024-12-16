"""Tests for CPU optimization"""
import pytest
from unittest.mock import Mock, patch, MagicMock
import psutil
from mevbot.core.cpu import CPUManager, CPUConfig

@pytest.fixture
def config():
    """Test configuration"""
    return {
        'network_cores': [0],
        'mempool_cores': [1],
        'worker_cores': [2, 3],
        'main_core': 4
    }

def test_cpu_config(config):
    """Test CPU configuration"""
    cpu_config = CPUConfig(
        network_cores=config['network_cores'],
        mempool_cores=config['mempool_cores'],
        worker_cores=config['worker_cores'],
        main_core=config['main_core']
    )
    assert cpu_config.network_cores == [0]
    assert cpu_config.mempool_cores == [1]
    assert cpu_config.worker_cores == [2, 3]
    assert cpu_config.main_core == 4

def test_cpu_config_validation():
    """Test CPU configuration validation"""
    # Test duplicate cores
    with pytest.raises(ValueError, match="Duplicate core assignments detected"):
        CPUConfig(
            network_cores=[0],
            mempool_cores=[0],  # Duplicate
            worker_cores=[1, 2],
            main_core=3
        )
    
    # Test invalid core number
    max_cores = psutil.cpu_count()
    with pytest.raises(ValueError, match=f"Core {max_cores} exceeds available cores"):
        CPUConfig(
            network_cores=[0],
            mempool_cores=[1],
            worker_cores=[2, max_cores],  # Invalid core
            main_core=3
        )

@patch('psutil.cpu_count')
def test_cpu_manager_initialization(mock_cpu_count, config):
    """Test CPU manager initialization"""
    mock_cpu_count.return_value = 8
    
    with patch('ctypes.CDLL') as mock_cdll, \
         patch('ctypes.util.find_library') as mock_find_library:
        mock_pthread = MagicMock()
        mock_numa = MagicMock()
        mock_cdll.side_effect = [mock_pthread, mock_numa]
        mock_find_library.side_effect = ['pthread', 'numa']
        
        manager = CPUManager(config)
        assert manager.initialized is False
        
        success = manager.initialize()
        assert success is True
        assert manager.initialized is True
        
        # Verify pthread was used for main thread pinning
        mock_pthread.pthread_setaffinity_np.assert_called()

@patch('psutil.cpu_count')
@patch('psutil.cpu_percent')
def test_optimal_core_selection(mock_cpu_percent, mock_cpu_count, config):
    """Test optimal core selection"""
    mock_cpu_count.return_value = 8
    mock_cpu_percent.return_value = [10, 20, 30, 40, 50, 60, 70, 80]
    
    manager = CPUManager(config)
    
    # Test network thread core selection
    core = manager.get_optimal_core('network')
    assert core == 0  # Only one network core available
    
    # Test worker thread core selection
    core = manager.get_optimal_core('worker')
    assert core == 2  # Core 2 has lower utilization than core 3
    
    # Test invalid thread type
    core = manager.get_optimal_core('invalid')
    assert core is None

@patch('psutil.cpu_count')
@patch('psutil.cpu_percent')
@patch('psutil.cpu_freq')
def test_core_stats(mock_cpu_freq, mock_cpu_percent, mock_cpu_count, config):
    """Test core statistics collection"""
    mock_cpu_count.return_value = 8
    mock_cpu_percent.return_value = [10, 20, 30, 40, 50, 60, 70, 80]
    
    class MockFreq:
        def __init__(self, current):
            self.current = current
    
    mock_cpu_freq.return_value = [
        MockFreq(2000),
        MockFreq(2100),
        MockFreq(2200),
        MockFreq(2300),
        MockFreq(2400),
        MockFreq(2500),
        MockFreq(2600),
        MockFreq(2700)
    ]
    
    manager = CPUManager(config)
    stats = manager.get_core_stats()
    
    # Verify stats structure
    assert 'network' in stats
    assert 'mempool' in stats
    assert 'worker' in stats
    assert 'main' in stats
    
    # Verify network core stats
    assert stats['network'][0]['utilization'] == 10
    assert stats['network'][0]['frequency'] == 2000
    
    # Verify worker core stats
    assert stats['worker'][2]['utilization'] == 30
    assert stats['worker'][2]['frequency'] == 2200
    assert stats['worker'][3]['utilization'] == 40
    assert stats['worker'][3]['frequency'] == 2300

@patch('psutil.cpu_count')
def test_numa_configuration(mock_cpu_count, config):
    """Test NUMA configuration"""
    mock_cpu_count.return_value = 8
    
    with patch('ctypes.CDLL') as mock_cdll, \
         patch('ctypes.util.find_library') as mock_find_library:
        mock_pthread = MagicMock()
        mock_numa = MagicMock()
        mock_numa.numa_num_configured_nodes.return_value = 2
        mock_cdll.side_effect = [mock_pthread, mock_numa]
        mock_find_library.side_effect = ['pthread', 'numa']
        
        manager = CPUManager(config)
        manager.initialize()
        
        # Verify NUMA node configuration
        mock_numa.numa_num_configured_nodes.assert_called_once()
        assert mock_numa.numa_run_on_node.call_count == len(config['network_cores'])

@patch('psutil.cpu_count')
def test_cpu_governor_setting(mock_cpu_count, config):
    """Test CPU governor configuration"""
    mock_cpu_count.return_value = 8
    
    with patch('os.path.exists') as mock_exists, \
         patch('builtins.open', create=True) as mock_open:
        mock_exists.return_value = True
        mock_file = MagicMock()
        mock_open.return_value.__enter__.return_value = mock_file
        
        manager = CPUManager(config)
        manager.initialize()
        
        # Verify governor was set to performance mode
        mock_file.write.assert_called_with("performance")
