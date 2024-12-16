package network

// DPDKConfig holds DPDK-specific configuration
type DPDKConfig struct {
	MemoryChannels  int
	SocketMemory   []int
	Cores          []int
	MainCore       int
	RxQueues       int
	TxQueues       int
	MemPoolSize    int
	MemCacheSize   int
	PmdPath        string
	HugePagesPath  string
	Interface      string
	Promiscuous    bool
}

// NetworkConfig holds network-related configuration
type NetworkConfig struct {
	DPDK *DPDKConfig
	Interface   string
	Port        int
	MaxPackets  int
	BufferSize  int
	QueueSize   int
	MetricsPort int
}
