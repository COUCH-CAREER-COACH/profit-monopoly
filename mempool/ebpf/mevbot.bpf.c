//go:build ignore

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

// Maximum entries in our maps
#define MAX_ENTRIES 10000

// Syscall latency threshold in nanoseconds
#define LATENCY_THRESHOLD 1000000 // 1ms

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, u32);              // tid
    __type(value, u64);            // timestamp
} start_times SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, u32);              // syscall number
    __type(value, struct latency); // latency stats
} latency_stats SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256KB ring buffer
} events SEC(".maps");

// Event structure sent to userspace
struct event {
    u32 pid;
    u32 tid;
    u64 duration_ns;
    u32 syscall_nr;
    char comm[16];
};

// Latency statistics
struct latency {
    u64 count;
    u64 total_ns;
    u64 max_ns;
};

// Entry point for syscall tracing
SEC("tracepoint/raw_syscalls/sys_enter")
int trace_enter(struct trace_event_raw_sys_enter *ctx)
{
    u64 ts = bpf_ktime_get_ns();
    u32 tid = bpf_get_current_pid_tgid();
    
    // Store entry timestamp
    bpf_map_update_elem(&start_times, &tid, &ts, BPF_ANY);
    return 0;
}

// Exit point for syscall tracing
SEC("tracepoint/raw_syscalls/sys_exit")
int trace_exit(struct trace_event_raw_sys_exit *ctx)
{
    u32 tid = bpf_get_current_pid_tgid();
    u64 *start_ts = bpf_map_lookup_elem(&start_times, &tid);
    if (!start_ts)
        return 0;

    // Calculate syscall duration
    u64 duration = bpf_ktime_get_ns() - *start_ts;
    
    // Only track slow syscalls
    if (duration > LATENCY_THRESHOLD) {
        struct event *e;
        e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
        if (e) {
            e->pid = bpf_get_current_pid_tgid() >> 32;
            e->tid = tid;
            e->duration_ns = duration;
            e->syscall_nr = ctx->id;
            bpf_get_current_comm(&e->comm, sizeof(e->comm));
            bpf_ringbuf_submit(e, 0);
        }

        // Update latency statistics
        struct latency *stats, new_stats = {};
        stats = bpf_map_lookup_elem(&latency_stats, &ctx->id);
        if (stats) {
            __sync_fetch_and_add(&stats->count, 1);
            __sync_fetch_and_add(&stats->total_ns, duration);
            if (duration > stats->max_ns)
                stats->max_ns = duration;
        } else {
            new_stats.count = 1;
            new_stats.total_ns = duration;
            new_stats.max_ns = duration;
            bpf_map_update_elem(&latency_stats, &ctx->id, &new_stats, BPF_ANY);
        }
    }

    // Cleanup
    bpf_map_delete_elem(&start_times, &tid);
    return 0;
}

// Network optimization: track socket operations
SEC("tracepoint/syscalls/sys_enter_socket")
int trace_socket(struct trace_event_raw_sys_enter *ctx)
{
    // Monitor socket creation for our process
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    char comm[16];
    bpf_get_current_comm(&comm, sizeof(comm));
    
    // Only track our process
    if (bpf_strncmp(comm, 11, "mevbot") == 0) {
        u64 ts = bpf_ktime_get_ns();
        u32 tid = bpf_get_current_pid_tgid();
        bpf_map_update_elem(&start_times, &tid, &ts, BPF_ANY);
    }
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
