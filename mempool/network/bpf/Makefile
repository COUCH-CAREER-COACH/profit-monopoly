CLANG ?= clang
CFLAGS := -O2 -g -Wall -Werror $(CFLAGS)

# BPF specific flags
BPF_CFLAGS := -target bpf \
              -D__TARGET_ARCH_x86 \
              -I/usr/include/$(shell uname -m)-linux-gnu \
              -I/usr/include/bpf \
              -I/usr/include

# Output directory
OUTPUT := .output

# Default target
.PHONY: all
all: $(OUTPUT)/network.o

# Create output directory
$(OUTPUT):
	mkdir -p $(OUTPUT)

# Compile BPF program
$(OUTPUT)/network.o: network.c | $(OUTPUT)
	$(CLANG) $(BPF_CFLAGS) $(CFLAGS) -c $< -o $@

# Clean target
.PHONY: clean
clean:
	rm -rf $(OUTPUT)
