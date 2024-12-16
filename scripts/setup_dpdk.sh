#!/bin/bash

# DPDK System Setup Script
# This script sets up the system for DPDK operation

set -e

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi

# Configuration
HUGEPAGE_SIZE=2048 # 2MB hugepages
HUGEPAGE_COUNT=4096 # Total 8GB of hugepages
DPDK_VERSION="21.11"
NIC_PCI_ADDRESS=$(lspci | grep Ethernet | head -n1 | cut -d' ' -f1)

# Install dependencies
apt-get update
apt-get install -y \
    build-essential \
    libnuma-dev \
    linux-headers-$(uname -r) \
    python3 \
    python3-pip \
    ninja-build \
    meson \
    pkg-config

# Setup hugepages
echo "Setting up hugepages..."
echo "vm.nr_hugepages=$HUGEPAGE_COUNT" > /etc/sysctl.d/hugepages.conf
sysctl -p /etc/sysctl.d/hugepages.conf

mkdir -p /mnt/huge
mount -t hugetlbfs nodev /mnt/huge
echo "nodev /mnt/huge hugetlbfs defaults 0 0" >> /etc/fstab

# Download and install DPDK
echo "Installing DPDK..."
wget https://fast.dpdk.org/rel/dpdk-$DPDK_VERSION.tar.xz
tar xf dpdk-$DPDK_VERSION.tar.xz
cd dpdk-$DPDK_VERSION

# Build DPDK
meson build
cd build
ninja
ninja install
ldconfig

# Bind NIC to DPDK-compatible driver
echo "Binding NIC to DPDK driver..."
if [ -n "$NIC_PCI_ADDRESS" ]; then
    # Unbind from current driver
    echo $NIC_PCI_ADDRESS > /sys/bus/pci/devices/$NIC_PCI_ADDRESS/driver/unbind
    
    # Bind to vfio-pci driver
    modprobe vfio-pci
    echo "vfio-pci" > /sys/bus/pci/devices/$NIC_PCI_ADDRESS/driver_override
    echo $NIC_PCI_ADDRESS > /sys/bus/pci/drivers/vfio-pci/bind
else
    echo "No NIC found!"
    exit 1
fi

# Configure CPU frequency scaling
echo "Configuring CPU frequency scaling..."
for cpu in /sys/devices/system/cpu/cpu[0-9]*; do
    echo performance > $cpu/cpufreq/scaling_governor
done

# Disable CPU C-states
echo "Disabling CPU C-states..."
for cpu in /sys/devices/system/cpu/cpu[0-9]*; do
    echo 0 > $cpu/power/pm_qos_resume_latency_us
done

# Setup IRQ affinity
echo "Setting up IRQ affinity..."
# Reserve CPUs 0-3 for DPDK
for irq in $(grep eth /proc/interrupts | cut -d: -f1); do
    echo 4-7 > /proc/irq/$irq/smp_affinity_list
done

echo "DPDK setup complete!"
echo "Please reboot the system for changes to take effect."
