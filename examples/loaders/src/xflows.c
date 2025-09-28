#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#define SAMPLE_RATE 1000

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
    __uint(max_entries, 1);
} sflow_sampler SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 23); // 8MB
} sflows SEC(".maps");
struct sflow_event {
    __u8 init_packet[128];
};

SEC("xdp") int xdp_main(struct xdp_md *ctx) {
    void *data = (void *)(long)ctx->data;

    __u32 key = 0;
    __u32 *counter = bpf_map_lookup_elem(&sflow_sampler, &key);
    if (counter) {
        __u32 new_value = *counter + 1;

        if (new_value == SAMPLE_RATE) {
            new_value = 0;
            struct sflow_event *event = bpf_ringbuf_reserve(&sflows, sizeof(struct sflow_event), 0);
            if (event) {
                bpf_probe_read_kernel(event->init_packet, 128, data);
                bpf_ringbuf_submit(event, 0);
            }
        } else if (new_value > SAMPLE_RATE) {
            new_value = 0;
        }

        *counter = new_value;
    }

    return XDP_PASS;
}

char __license[] SEC("license") = "Dual MIT/GPL";