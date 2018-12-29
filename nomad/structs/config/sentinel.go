package config

// SentinelConfig is configuration specific to Sentinel
type SentinelConfig struct {
	// Imports are the configured imports
	Imports []*SentinelImport `hcl:"import,expand"`
}

// SentinelImport is used per configured import
type SentinelImport struct {
	Name string   `hcl:",key"`
	Path string   `hcl:"path"`
	Args []string `hcl:"args"`
}

// Merge is used to merge two Sentinel configs together. The settings from the input always take precedence.
func (a *SentinelConfig) Merge(b *SentinelConfig) *SentinelConfig {
	result := *a
	if len(b.Imports) > 0 {
		result.Imports = append(result.Imports, b.Imports...)
	}
	return &result
}
