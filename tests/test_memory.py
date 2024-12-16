"""Tests for memory optimization module"""
import pytest
import mmap
from unittest.mock import patch, mock_open, MagicMock
from mevbot.core.memory import MemoryManager, HugePagesConfig

@pytest.fixture
def memory_config():
    """Memory configuration fixture"""
    return {
        'huge_page_size': 2048,  # 2MB
        'huge_pages_count': 1024,
        'huge_pages_mount': '/dev/hugepages',
        'min_free_huge_pages': 64
    }

@pytest.fixture
def memory_manager(memory_config):
    """Memory manager fixture"""
    return MemoryManager(memory_config)

def test_huge_pages_config():
    """Test huge pages configuration validation"""
    # Valid configuration
    config = HugePagesConfig(2048, 1024, '/dev/hugepages', 64)
    assert config.page_size == 2048
    assert config.pages_count == 1024
    
    # Invalid page size
    with pytest.raises(ValueError):
        HugePagesConfig(4096, 1024, '/dev/hugepages', 64)
        
    # Invalid pages count
    with pytest.raises(ValueError):
        HugePagesConfig(2048, 0, '/dev/hugepages', 64)
        
    # Invalid min free pages
    with pytest.raises(ValueError):
        HugePagesConfig(2048, 1024, '/dev/hugepages', 1025)

@patch('subprocess.run')
@patch('pathlib.Path.mkdir')
@patch('builtins.open', new_callable=mock_open)
def test_memory_manager_initialization(mock_file, mock_mkdir, mock_run, memory_manager):
    """Test memory manager initialization"""
    # Mock /proc/mounts content
    mock_file.return_value.__enter__.return_value.readlines.return_value = [
        'hugetlbfs /dev/hugepages hugetlbfs rw,relatime 0 0'
    ]
    
    # Mock /proc/meminfo content
    mock_meminfo = (
        'HugePages_Total: 1024\n'
        'HugePages_Free: 1024\n'
        'HugePages_Rsvd: 0\n'
        'HugePages_Surp: 0\n'
    )
    mock_file.return_value.__enter__.return_value.read.return_value = mock_meminfo
    
    assert memory_manager.initialize() is True
    assert memory_manager.initialized is True
    
    # Verify sysctl calls
    mock_run.assert_any_call(['sysctl', '-w', 'vm.nr_hugepages=1024'], check=True)
    mock_run.assert_any_call(['sysctl', '-w', 'vm.nr_overcommit_hugepages=64'], check=True)

@patch('mevbot.core.memory.mmap')
@patch('builtins.open', new_callable=mock_open)
def test_memory_region_allocation(mock_file, mock_mmap, memory_manager):
    """Test memory region allocation"""
    # Mock initialization
    memory_manager.initialized = True
    
    # Mock mmap module constants
    mock_mmap.MAP_SHARED = 1
    mock_mmap.MAP_HUGETLB = 0x40000  # Linux value
    
    # Mock available pages with proper format
    mock_meminfo_lines = [
        'HugePages_Total:    1024\n',
        'HugePages_Free:     1024\n',
        'HugePages_Rsvd:     0\n',
        'HugePages_Surp:     0\n',
        'Hugepagesize:       2048 kB\n'
    ]
    
    # Create a class to handle file iteration
    class MockFileIterator:
        def __init__(self, lines):
            self.lines = lines
            
        def __iter__(self):
            return iter(self.lines)
            
    mock_file_handle = mock_file.return_value.__enter__.return_value
    mock_file_iterator = MockFileIterator(mock_meminfo_lines)
    mock_file_handle.__iter__ = lambda self: iter(mock_meminfo_lines)
    
    # Mock mmap
    mock_mmap_obj = MagicMock()
    mock_mmap.mmap.return_value = mock_mmap_obj
    
    # Mock file creation in hugetlbfs
    mock_file_handle.fileno.return_value = 42
    
    # Allocate region
    region = memory_manager.allocate_region('test_region', 128)  # 128MB
    assert region is not None
    assert 'test_region' in memory_manager._mapped_regions
    
    # Verify mmap call
    mock_mmap.mmap.assert_called_once()
    assert mock_mmap.mmap.call_args[0][1] == 64 * 2048 * 1024  # 64 pages of 2MB each

def test_memory_stats(memory_manager):
    """Test memory statistics"""
    # Mock initialization
    memory_manager.initialized = True
    
    # Mock mapped regions
    mock_region = MagicMock()
    mock_region.size.return_value = 128 * 1024 * 1024  # 128MB
    memory_manager._mapped_regions = {'test_region': mock_region}
    
    # Mock /proc/meminfo content
    meminfo_content = (
        'HugePages_Total: 1024\n'
        'HugePages_Free: 960\n'
        'HugePages_Rsvd: 0\n'
        'HugePages_Surp: 0\n'
    )
    
    with patch('builtins.open', mock_open(read_data=meminfo_content)):
        stats = memory_manager.get_stats()
        
        assert stats['total_pages'] == 1024
        assert stats['free_pages'] == 960
        assert stats['mapped_regions'] == 1
        assert stats['mapped_regions_size_mb'] == 128

@patch('pathlib.Path.unlink')
@patch('pathlib.Path.exists')
def test_memory_region_free(mock_exists, mock_unlink, memory_manager):
    """Test memory region freeing"""
    # Mock initialization
    memory_manager.initialized = True
    
    # Mock mapped region
    mock_region = MagicMock()
    memory_manager._mapped_regions['test_region'] = mock_region
    mock_exists.return_value = True
    
    # Free region
    assert memory_manager.free_region('test_region') is True
    mock_region.close.assert_called_once()
    mock_unlink.assert_called_once()
    assert 'test_region' not in memory_manager._mapped_regions

def test_memory_region_allocation_insufficient_pages(memory_manager):
    """Test memory region allocation with insufficient pages"""
    # Mock initialization
    memory_manager.initialized = True
    
    # Mock available pages (less than minimum + required)
    meminfo_content = (
        'HugePages_Total: 1024\n'
        'HugePages_Free: 32\n'
        'HugePages_Rsvd: 0\n'
        'HugePages_Surp: 0\n'
    )
    
    with patch('builtins.open', mock_open(read_data=meminfo_content)):
        region = memory_manager.allocate_region('test_region', 1024)  # Try to allocate 1GB
        assert region is None  # Allocation should fail
