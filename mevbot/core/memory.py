"""Memory optimization using huge pages"""
import os
import mmap
import ctypes
import logging
import psutil
from typing import Dict, Optional, List
from dataclasses import dataclass
import subprocess
from pathlib import Path

logger = logging.getLogger(__name__)

@dataclass
class HugePagesConfig:
    """Huge pages configuration"""
    page_size: int  # Size in KB (typically 2048 or 1048576)
    pages_count: int  # Number of pages to allocate
    mount_point: str  # Where to mount hugetlbfs
    min_free_pages: int  # Minimum free pages to maintain
    
    def __post_init__(self):
        """Validate configuration"""
        if self.page_size not in [2048, 1048576]:  # 2MB or 1GB
            raise ValueError("Page size must be 2048KB (2MB) or 1048576KB (1GB)")
        if self.pages_count < 1:
            raise ValueError("Must allocate at least one huge page")
        if self.min_free_pages >= self.pages_count:
            raise ValueError("Minimum free pages must be less than total pages")

class MemoryManager:
    """Manages memory optimization using huge pages"""
    
    def __init__(self, config: Dict):
        """Initialize memory manager"""
        self.config = HugePagesConfig(
            page_size=config.get('huge_page_size', 2048),  # Default to 2MB pages
            pages_count=config.get('huge_pages_count', 1024),
            mount_point=config.get('huge_pages_mount', '/dev/hugepages'),
            min_free_pages=config.get('min_free_huge_pages', 64)
        )
        self.initialized = False
        self._mapped_regions: Dict[str, mmap.mmap] = {}
        
    def initialize(self) -> bool:
        """Initialize huge pages"""
        if self.initialized:
            return True
            
        try:
            # Configure kernel parameters
            self._configure_kernel_params()
            
            # Mount hugetlbfs if not already mounted
            self._mount_hugetlbfs()
            
            # Allocate huge pages
            self._allocate_huge_pages()
            
            self.initialized = True
            logger.info("Memory optimization initialized successfully")
            return True
            
        except Exception as e:
            logger.error(f"Failed to initialize memory optimization: {e}")
            return False
            
    def _configure_kernel_params(self):
        """Configure kernel parameters for huge pages"""
        try:
            # Set vm.nr_hugepages
            self._write_sysctl('vm.nr_hugepages', str(self.config.pages_count))
            
            # Set vm.nr_overcommit_hugepages for dynamic allocation
            self._write_sysctl('vm.nr_overcommit_hugepages', '64')
            
            # Disable transparent huge pages for better control
            self._write_to_file('/sys/kernel/mm/transparent_hugepage/enabled', 'never')
            
            logger.info("Kernel parameters configured successfully")
            
        except Exception as e:
            logger.error(f"Failed to configure kernel parameters: {e}")
            raise
            
    def _write_sysctl(self, param: str, value: str):
        """Write sysctl parameter"""
        try:
            subprocess.run(['sysctl', '-w', f'{param}={value}'], check=True)
        except subprocess.CalledProcessError as e:
            logger.error(f"Failed to set sysctl parameter {param}: {e}")
            raise
            
    def _write_to_file(self, path: str, value: str):
        """Write value to file"""
        try:
            with open(path, 'w') as f:
                f.write(value)
        except Exception as e:
            logger.error(f"Failed to write to {path}: {e}")
            raise
            
    def _mount_hugetlbfs(self):
        """Mount hugetlbfs if not already mounted"""
        try:
            mount_point = Path(self.config.mount_point)
            
            # Create mount point if it doesn't exist
            mount_point.mkdir(parents=True, exist_ok=True)
            
            # Check if already mounted
            with open('/proc/mounts', 'r') as f:
                if any(line.split()[1] == str(mount_point) for line in f):
                    logger.info(f"hugetlbfs already mounted at {mount_point}")
                    return
                    
            # Mount hugetlbfs
            subprocess.run([
                'mount', '-t', 'hugetlbfs',
                '-o', f'pagesize={self.config.page_size}k',
                'none', str(mount_point)
            ], check=True)
            
            logger.info(f"hugetlbfs mounted at {mount_point}")
            
        except Exception as e:
            logger.error(f"Failed to mount hugetlbfs: {e}")
            raise
            
    def _allocate_huge_pages(self):
        """Allocate huge pages"""
        try:
            # Read current allocation
            with open('/proc/meminfo', 'r') as f:
                meminfo = {}
                for line in f:
                    if 'HugePages' in line:
                        key, value = line.split(':')
                        meminfo[key.strip()] = value.strip()
                current = int(meminfo.get('HugePages_Total', '0').split()[0])
                
            if current < self.config.pages_count:
                pages_to_allocate = self.config.pages_count - current
                logger.info(f"Allocating {pages_to_allocate} additional huge pages")
                self._write_sysctl('vm.nr_hugepages', str(self.config.pages_count))
                
        except Exception as e:
            logger.error(f"Failed to allocate huge pages: {e}")
            raise
            
    def allocate_region(self, name: str, size_mb: int) -> Optional[mmap.mmap]:
        """Allocate a memory region using huge pages"""
        if not self.initialized:
            logger.error("Memory manager not initialized")
            return None
            
        try:
            # Calculate required pages
            pages_needed = (size_mb * 1024 * 1024) // (self.config.page_size * 1024)
            if pages_needed == 0:
                pages_needed = 1
                
            # Check available pages
            available = self._get_available_pages()
            if available < pages_needed + self.config.min_free_pages:
                logger.error(f"Not enough huge pages available. Need {pages_needed}, have {available}")
                return None
                
            # Create file in hugetlbfs
            file_path = Path(self.config.mount_point) / name
            with open(file_path, 'wb+') as f:
                # Allocate the memory
                mem = mmap.mmap(
                    f.fileno(), pages_needed * self.config.page_size * 1024,
                    flags=mmap.MAP_SHARED | mmap.MAP_HUGETLB
                )
                
            self._mapped_regions[name] = mem
            logger.info(f"Allocated {size_mb}MB using {pages_needed} huge pages for {name}")
            return mem
            
        except Exception as e:
            logger.error(f"Failed to allocate memory region {name}: {e}")
            return None
            
    def free_region(self, name: str) -> bool:
        """Free a memory region"""
        try:
            if name in self._mapped_regions:
                mem = self._mapped_regions[name]
                mem.close()
                
                # Remove the file
                file_path = Path(self.config.mount_point) / name
                if file_path.exists():
                    file_path.unlink()
                    
                del self._mapped_regions[name]
                logger.info(f"Freed memory region {name}")
                return True
                
            return False
            
        except Exception as e:
            logger.error(f"Failed to free memory region {name}: {e}")
            return False
            
    def _get_available_pages(self) -> int:
        """Get number of available huge pages"""
        try:
            with open('/proc/meminfo', 'r') as f:
                meminfo = {}
                for line in f:
                    if 'HugePages' in line:
                        key, value = line.split(':')
                        meminfo[key.strip()] = value.strip()
                return int(meminfo.get('HugePages_Free', '0').split()[0])
        except Exception as e:
            logger.error(f"Failed to get available huge pages: {e}")
            return 0
            
    def get_stats(self) -> Dict:
        """Get memory statistics"""
        try:
            with open('/proc/meminfo', 'r') as f:
                meminfo = {}
                for line in f:
                    if 'HugePages' in line:
                        key, value = line.split(':')
                        meminfo[key.strip()] = value.strip()
                
            stats = {
                'total_pages': int(meminfo.get('HugePages_Total', '0').split()[0]),
                'free_pages': int(meminfo.get('HugePages_Free', '0').split()[0]),
                'reserved_pages': int(meminfo.get('HugePages_Rsvd', '0').split()[0]),
                'surplus_pages': int(meminfo.get('HugePages_Surp', '0').split()[0]),
                'page_size_kb': self.config.page_size,
                'mapped_regions': len(self._mapped_regions),
                'mapped_regions_size_mb': sum(
                    mem.size() // (1024 * 1024)
                    for mem in self._mapped_regions.values()
                )
            }
            
            return stats
            
        except Exception as e:
            logger.error(f"Failed to get memory stats: {e}")
            return {}
