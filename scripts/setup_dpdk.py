#!/usr/bin/env python3
"""
Setup script for DPDK configuration
This script:
1. Configures huge pages
2. Binds network interfaces to DPDK-compatible drivers
3. Sets up CPU affinity
"""
import os
import sys
import subprocess
import psutil
from pathlib import Path
import argparse
import logging
from pyroute2 import IPRoute

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def setup_hugepages(pages=1024, size=2048):
    """Configure huge pages for DPDK"""
    try:
        # Calculate total memory needed
        total_memory = pages * size * 1024  # Convert to bytes
        available_memory = psutil.virtual_memory().available
        
        if total_memory > available_memory * 0.8:  # Don't use more than 80% of available memory
            pages = int((available_memory * 0.8) / (size * 1024))
            logger.warning(f"Reducing huge pages to {pages} due to memory constraints")
            
        # Write number of huge pages
        hugepage_path = Path("/sys/kernel/mm/hugepages/hugepages-2048kB/nr_hugepages")
        if not hugepage_path.exists():
            logger.error("Huge pages not supported on this system")
            return False
            
        with open(hugepage_path, 'w') as f:
            f.write(str(pages))
            
        # Mount huge pages if not already mounted
        if not os.path.ismount("/dev/hugepages"):
            os.makedirs("/dev/hugepages", exist_ok=True)
            subprocess.run(["mount", "-t", "hugetlbfs", "nodev", "/dev/hugepages"])
            
        logger.info(f"Successfully configured {pages} huge pages of {size}KB")
        return True
        
    except Exception as e:
        logger.error(f"Failed to setup huge pages: {e}")
        return False

def bind_network_interfaces(interfaces):
    """Bind network interfaces to DPDK-compatible driver"""
    try:
        ip = IPRoute()
        for interface in interfaces:
            # Get interface index
            idx = ip.link_lookup(ifname=interface)[0]
            
            # Bring interface down
            ip.link('set', index=idx, state='down')
            
            # Unbind from current driver
            subprocess.run(["dpdk-devbind", "--unbind", interface])
            
            # Bind to DPDK-compatible driver (uio_pci_generic or vfio-pci)
            subprocess.run(["dpdk-devbind", "--bind=uio_pci_generic", interface])
            
        logger.info(f"Successfully bound interfaces: {interfaces}")
        return True
        
    except Exception as e:
        logger.error(f"Failed to bind network interfaces: {e}")
        return False
    finally:
        ip.close()

def setup_cpu_affinity(cores):
    """Set CPU affinity for DPDK processes"""
    try:
        # Verify cores exist
        available_cores = psutil.cpu_count()
        if max(cores) >= available_cores:
            logger.error(f"Requested cores {cores} exceed available cores (0-{available_cores-1})")
            return False
            
        # Set CPU governor to performance
        for core in cores:
            governor_path = f"/sys/devices/system/cpu/cpu{core}/cpufreq/scaling_governor"
            if os.path.exists(governor_path):
                with open(governor_path, 'w') as f:
                    f.write("performance")
                    
        # Isolate cores from system scheduler
        isolated_cores = ",".join(map(str, cores))
        subprocess.run(["systemctl", "set-property", "system.slice", f"AllowedCPUs=0-{available_cores-1}"])
        subprocess.run(["systemctl", "set-property", "user.slice", f"AllowedCPUs=0-{available_cores-1}"])
        subprocess.run(["systemctl", "set-property", "init.scope", f"AllowedCPUs=0-{available_cores-1}"])
        
        logger.info(f"Successfully configured CPU affinity for cores: {cores}")
        return True
        
    except Exception as e:
        logger.error(f"Failed to setup CPU affinity: {e}")
        return False

def main():
    parser = argparse.ArgumentParser(description="Setup DPDK environment")
    parser.add_argument("--hugepages", type=int, default=1024,
                      help="Number of huge pages to allocate")
    parser.add_argument("--interfaces", nargs="+", required=True,
                      help="Network interfaces to bind to DPDK")
    parser.add_argument("--cores", nargs="+", type=int, required=True,
                      help="CPU cores to use for DPDK")
                      
    args = parser.parse_args()
    
    # Check if running as root
    if os.geteuid() != 0:
        logger.error("This script must be run as root")
        sys.exit(1)
        
    # Setup components
    success = True
    success &= setup_hugepages(args.hugepages)
    success &= bind_network_interfaces(args.interfaces)
    success &= setup_cpu_affinity(args.cores)
    
    if success:
        logger.info("DPDK environment setup completed successfully")
    else:
        logger.error("DPDK environment setup failed")
        sys.exit(1)

if __name__ == "__main__":
    main()
