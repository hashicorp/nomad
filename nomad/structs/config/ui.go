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

	// Label configures UI label styles
	Label *LabelUIConfig `hcl:"label"`
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

// Label configures UI label styles
type LabelUIConfig struct {
	Text            string `hcl:"text"`
	BackgroundColor string `hcl:"background_color"`
	TextColor       string `hcl:"text_color"`
}

// DefaultUIConfig returns the canonical defaults for the Nomad
// `ui` configuration.
func DefaultUIConfig() *UIConfig {
	return &UIConfig{
		Enabled: true,
		Consul:  &ConsulUIConfig{},
		Vault:   &VaultUIConfig{},
		Label:   &LabelUIConfig{},
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
	result.Label = result.Label.Merge(other.Label)

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

// Copy returns a copy of this Label UI config.
func (old *LabelUIConfig) Copy() *LabelUIConfig {
	if old == nil {
		return nil
	}

	nc := new(LabelUIConfig)
	*nc = *old
	return nc
}

// Merge returns a new Label UI configuration by merging another Label UI
// configuration into this one
func (old *LabelUIConfig) Merge(other *LabelUIConfig) *LabelUIConfig {
	result := old.Copy()
	if result == nil {
		result = &LabelUIConfig{}
	}
	if other == nil {
		return result
	}

	if other.Text != "" {
		result.Text = other.Text
	}
	if other.BackgroundColor != "" {
		result.BackgroundColor = other.BackgroundColor
	}
	if other.TextColor != "" {
		result.TextColor = other.TextColor
	}
	return result
}
