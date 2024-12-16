package flashloan

// ProviderType represents different flash loan providers
type ProviderType int

const (
	ProviderAave ProviderType = iota
	ProviderBalancer
	ProviderDyDx
)
