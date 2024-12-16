//go:build ignore

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

// Map to track syscall statistics
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1000);
    __type(key, __u32);    // syscall ID
    __type(value, __u64);  // count
} syscall_map SEC(".maps");

// Map for syscall latency tracking
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1000);
    __type(key, __u32);    // syscall ID
    __type(value, __u64);  // cumulative latency
} latency_map SEC(".maps");

SEC("socket")
int network_filter(struct __sk_buff *skb) {
    // Track read/write syscalls
    __u64 now = bpf_ktime_get_ns();
    __u32 syscall_id = bpf_get_current_pid_tgid() >> 32;

    // Update syscall count
    __u64 *count = bpf_map_lookup_elem(&syscall_map, &syscall_id);
    if (count) {
        __u64 new_count = *count + 1;
        bpf_map_update_elem(&syscall_map, &syscall_id, &new_count, BPF_ANY);
    } else {
        __u64 new_count = 1;
        bpf_map_update_elem(&syscall_map, &syscall_id, &new_count, BPF_ANY);
    }

    // Track latency
    __u64 *latency = bpf_map_lookup_elem(&latency_map, &syscall_id);
    if (latency) {
        __u64 new_latency = *latency + bpf_ktime_get_ns() - now;
        bpf_map_update_elem(&latency_map, &syscall_id, &new_latency, BPF_ANY);
    } else {
        __u64 new_latency = bpf_ktime_get_ns() - now;
        bpf_map_update_elem(&latency_map, &syscall_id, &new_latency, BPF_ANY);
    }

    // Always accept the packet - we're just monitoring syscalls
    return 1;
}

char LICENSE[] SEC("license") = "GPL";
