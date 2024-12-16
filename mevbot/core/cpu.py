"""CPU optimization and pinning"""
import os
import psutil
import logging
from typing import List, Dict, Optional
from dataclasses import dataclass
import ctypes
import ctypes.util

logger = logging.getLogger(__name__)

@dataclass
class CPUConfig:
    """CPU configuration"""
    network_cores: List[int]  # Cores for network processing
    mempool_cores: List[int]  # Cores for memory pool management
    worker_cores: List[int]   # Cores for worker threads
    main_core: int           # Core for main thread
    
    def __post_init__(self):
        """Validate CPU configuration"""
        all_cores = (
            self.network_cores +
            self.mempool_cores +
            self.worker_cores +
            [self.main_core]
        )
        
        # Check for duplicates
        if len(all_cores) != len(set(all_cores)):
            raise ValueError("Duplicate core assignments detected")
            
        # Check core availability
        max_cores = psutil.cpu_count()
        if max(all_cores) >= max_cores:
            raise ValueError(f"Core {max(all_cores)} exceeds available cores (0-{max_cores-1})")

class CPUManager:
    """Manages CPU optimization and pinning"""
    
    def __init__(self, config: Dict):
        """Initialize CPU manager"""
        self.config = CPUConfig(
            network_cores=config.get('network_cores', [0]),
            mempool_cores=config.get('mempool_cores', [1]),
            worker_cores=config.get('worker_cores', [2, 3]),
            main_core=config.get('main_core', 4)
        )
        self.initialized = False
        self._load_libraries()
        
    def _load_libraries(self):
        """Load required libraries"""
        try:
            # Load pthread library for thread management
            pthread_path = ctypes.util.find_library('pthread')
            if pthread_path:
                self.pthread = ctypes.CDLL(pthread_path)
            else:
                logger.warning("pthread library not found")
                self.pthread = None
                
            # Load numa library if available
            numa_path = ctypes.util.find_library('numa')
            if numa_path:
                self.numa = ctypes.CDLL(numa_path)
            else:
                logger.warning("numa library not found")
                self.numa = None
                
        except Exception as e:
            logger.error(f"Failed to load libraries: {e}")
            self.pthread = None
            self.numa = None
            
    def initialize(self) -> bool:
        """Initialize CPU optimization"""
        if self.initialized:
            return True
            
        try:
            # Set CPU governor to performance mode
            self._set_cpu_governor()
            
            # Configure NUMA if available
            if self.numa:
                self._configure_numa()
                
            # Pin main thread
            self.pin_thread_to_core(os.getpid(), self.config.main_core)
            
            self.initialized = True
            logger.info("CPU optimization initialized successfully")
            return True
            
        except Exception as e:
            logger.error(f"Failed to initialize CPU optimization: {e}")
            return False
            
    def _set_cpu_governor(self):
        """Set CPU governor to performance mode"""
        try:
            all_cores = (
                self.config.network_cores +
                self.config.mempool_cores +
                self.config.worker_cores +
                [self.config.main_core]
            )
            
            for core in all_cores:
                governor_path = f"/sys/devices/system/cpu/cpu{core}/cpufreq/scaling_governor"
                if os.path.exists(governor_path):
                    with open(governor_path, 'w') as f:
                        f.write("performance")
                        
        except Exception as e:
            logger.warning(f"Failed to set CPU governor: {e}")
            
    def _configure_numa(self):
        """Configure NUMA node assignments"""
        if not self.numa:
            return
            
        try:
            # Get number of NUMA nodes
            num_nodes = self.numa.numa_num_configured_nodes()
            
            # Distribute cores across NUMA nodes
            for core in self.config.network_cores:
                node = core % num_nodes
                self.numa.numa_run_on_node(node)
                
        except Exception as e:
            logger.warning(f"Failed to configure NUMA: {e}")
            
    def pin_thread_to_core(self, thread_id: int, core_id: int) -> bool:
        """Pin a thread to a specific CPU core"""
        try:
            if not self.pthread:
                logger.warning("pthread library not available")
                return False
                
            # Create CPU set
            cpu_set = ctypes.c_uint64(1 << core_id)
            
            # Set CPU affinity
            result = self.pthread.pthread_setaffinity_np(
                thread_id,
                ctypes.sizeof(cpu_set),
                ctypes.byref(cpu_set)
            )
            
            if result != 0:
                logger.error(f"Failed to pin thread {thread_id} to core {core_id}")
                return False
                
            logger.info(f"Successfully pinned thread {thread_id} to core {core_id}")
            return True
            
        except Exception as e:
            logger.error(f"Error pinning thread to core: {e}")
            return False
            
    def get_optimal_core(self, thread_type: str) -> Optional[int]:
        """Get optimal core for a new thread based on type"""
        try:
            if thread_type == 'network':
                cores = self.config.network_cores
            elif thread_type == 'mempool':
                cores = self.config.mempool_cores
            elif thread_type == 'worker':
                cores = self.config.worker_cores
            else:
                logger.error(f"Unknown thread type: {thread_type}")
                return None
                
            # Get core utilization
            core_usage = {
                core: psutil.cpu_percent(percpu=True)[core]
                for core in cores
            }
            
            # Return core with lowest utilization
            return min(core_usage.items(), key=lambda x: x[1])[0]
            
        except Exception as e:
            logger.error(f"Error getting optimal core: {e}")
            return None
            
    def get_core_stats(self) -> Dict:
        """Get statistics for managed cores"""
        try:
            cpu_percent = psutil.cpu_percent(percpu=True)
            cpu_freq = psutil.cpu_freq(percpu=True)
            
            stats = {}
            all_cores = (
                ('network', self.config.network_cores),
                ('mempool', self.config.mempool_cores),
                ('worker', self.config.worker_cores),
                ('main', [self.config.main_core])
            )
            
            for core_type, cores in all_cores:
                stats[core_type] = {
                    core: {
                        'utilization': cpu_percent[core],
                        'frequency': cpu_freq[core].current if cpu_freq else None
                    }
                    for core in cores
                }
                
            return stats
            
        except Exception as e:
            logger.error(f"Error getting core stats: {e}")
            return {}
