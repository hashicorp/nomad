package config

// UIConfig contains the operator configuration of the web UI
// Note:
// before extending this configuration, consider reviewing NMD-125
type UIConfig struct {

	// Enabled is used to enable the web UI
	Enabled bool `hcl:"enabled"`

	// Consul configures deep links for Consul UI
	Consul *ConsulUIConfig `hcl:"consul"`

	// Vault configures deep links for Vault UI
	Vault *VaultUIConfig `hcl:"vault"`
}

// ConsulUIConfig configures deep links to this cluster's Consul
type ConsulUIConfig struct {

	// BaseUIURL provides the full base URL to the UI, ex:
	// https://consul.example.com:8500/ui/
	BaseUIURL string `hcl:"ui_url"`
}

// VaultUIConfig configures deep links to this cluster's Vault
type VaultUIConfig struct {
	// BaseUIURL provides the full base URL to the UI, ex:
	// https://vault.example.com:8200/ui/
	BaseUIURL string `hcl:"ui_url"`
}

// DefaultUIConfig returns the canonical defaults for the Nomad
// `ui` configuration.
func DefaultUIConfig() *UIConfig {
	return &UIConfig{
		Enabled: true,
		Consul:  &ConsulUIConfig{},
		Vault:   &VaultUIConfig{},
	}
}

// Copy returns a copy of this UI config.
func (old *UIConfig) Copy() *UIConfig {
	if old == nil {
		return nil
	}

	nc := new(UIConfig)
	*nc = *old

	if old.Consul != nil {
		nc.Consul = old.Consul.Copy()
	}
	if old.Vault != nil {
		nc.Vault = old.Vault.Copy()
	}
	return nc
}

// Merge returns a new UI configuration by merging another UI
// configuration into this one
func (old *UIConfig) Merge(other *UIConfig) *UIConfig {
	result := old.Copy()
	if other == nil {
		return result
	}

	result.Enabled = other.Enabled
	result.Consul = result.Consul.Merge(other.Consul)
	result.Vault = result.Vault.Merge(other.Vault)

	return result
}

// Copy returns a copy of this Consul UI config.
func (old *ConsulUIConfig) Copy() *ConsulUIConfig {
	if old == nil {
		return nil
	}

	nc := new(ConsulUIConfig)
	*nc = *old
	return nc
}

// Merge returns a new Consul UI configuration by merging another Consul UI
// configuration into this one
func (old *ConsulUIConfig) Merge(other *ConsulUIConfig) *ConsulUIConfig {
	result := old.Copy()
	if result == nil {
		result = &ConsulUIConfig{}
	}
	if other == nil {
		return result
	}

	if other.BaseUIURL != "" {
		result.BaseUIURL = other.BaseUIURL
	}
	return result
}

// Copy returns a copy of this Vault UI config.
func (old *VaultUIConfig) Copy() *VaultUIConfig {
	if old == nil {
		return nil
	}

	nc := new(VaultUIConfig)
	*nc = *old
	return nc
}

// Merge returns a new Vault UI configuration by merging another Vault UI
// configuration into this one
func (old *VaultUIConfig) Merge(other *VaultUIConfig) *VaultUIConfig {
	result := old.Copy()
	if result == nil {
		result = &VaultUIConfig{}
	}
	if other == nil {
		return result
	}

	if other.BaseUIURL != "" {
		result.BaseUIURL = other.BaseUIURL
	}
	return result
}
