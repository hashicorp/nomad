// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"fmt"
	"slices"
	"strings"
)

// UIConfig contains the operator configuration of the web UI
// Note:
// before extending this configuration, consider reviewing NMD-125
type UIConfig struct {

	// Enabled is used to enable the web UI
	Enabled bool `hcl:"enabled"`

	// ContentSecurityPolicy is used to configure the CSP header
	ContentSecurityPolicy *ContentSecurityPolicy `hcl:"content_security_policy"`

	// Consul configures deep links for Consul UI
	Consul *ConsulUIConfig `hcl:"consul"`

	// Vault configures deep links for Vault UI
	Vault *VaultUIConfig `hcl:"vault"`

	// Label configures UI label styles
	Label *LabelUIConfig `hcl:"label"`
}

// only covers the elements of
// https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP we need or care about
type ContentSecurityPolicy struct {
	ConnectSrc     []string `hcl:"connect_src"`
	DefaultSrc     []string `hcl:"default_src"`
	FormAction     []string `hcl:"form_action"`
	FrameAncestors []string `hcl:"frame_ancestors"`
	ImgSrc         []string `hcl:"img_src"`
	ScriptSrc      []string `hcl:"script_src"`
	StyleSrc       []string `hcl:"style_src"`
}

// Copy returns a copy of this Vault UI config.
func (old *ContentSecurityPolicy) Copy() *ContentSecurityPolicy {
	if old == nil {
		return nil
	}

	nc := new(ContentSecurityPolicy)
	*nc = *old
	nc.ConnectSrc = slices.Clone(old.ConnectSrc)
	nc.DefaultSrc = slices.Clone(old.DefaultSrc)
	nc.FormAction = slices.Clone(old.FormAction)
	nc.FrameAncestors = slices.Clone(old.FrameAncestors)
	nc.ImgSrc = slices.Clone(old.ImgSrc)
	nc.ScriptSrc = slices.Clone(old.ScriptSrc)
	nc.StyleSrc = slices.Clone(old.StyleSrc)
	return nc
}

func (csp *ContentSecurityPolicy) String() string {
	return fmt.Sprintf("default-src %s; connect-src %s; img-src %s; script-src %s; style-src %s; form-action %s; frame-ancestors %s", strings.Join(csp.DefaultSrc, " "), strings.Join(csp.ConnectSrc, " "), strings.Join(csp.ImgSrc, " "), strings.Join(csp.ScriptSrc, " "), strings.Join(csp.StyleSrc, " "), strings.Join(csp.FormAction, " "), strings.Join(csp.FrameAncestors, " "))
}

func (csp *ContentSecurityPolicy) Merge(other *ContentSecurityPolicy) *ContentSecurityPolicy {
	result := csp.Copy()
	if result == nil {
		result = &ContentSecurityPolicy{}
	}
	if other == nil {
		return result
	}

	if len(other.ConnectSrc) > 0 {
		result.ConnectSrc = other.ConnectSrc
	}
	if len(other.DefaultSrc) > 0 {
		result.DefaultSrc = other.DefaultSrc
	}
	if len(other.FormAction) > 0 {
		result.FormAction = other.FormAction
	}
	if len(other.FrameAncestors) > 0 {
		result.FrameAncestors = other.FrameAncestors
	}
	if len(other.ImgSrc) > 0 {
		result.ImgSrc = other.ImgSrc
	}
	if len(other.ScriptSrc) > 0 {
		result.ScriptSrc = other.ScriptSrc
	}
	if len(other.StyleSrc) > 0 {
		result.StyleSrc = other.StyleSrc
	}

	return result

}

func DefaultCSPConfig() *ContentSecurityPolicy {
	return &ContentSecurityPolicy{
		ConnectSrc:     []string{"*"},
		DefaultSrc:     []string{"'none'"},
		FormAction:     []string{"'none'"},
		FrameAncestors: []string{"'none'"},
		ImgSrc:         []string{"'self'", "data:"},
		ScriptSrc:      []string{"'self'"},
		StyleSrc:       []string{"'self'", "'unsafe-inline'"},
	}
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
		Enabled:               true,
		Consul:                &ConsulUIConfig{},
		Vault:                 &VaultUIConfig{},
		Label:                 &LabelUIConfig{},
		ContentSecurityPolicy: DefaultCSPConfig(),
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
	result.ContentSecurityPolicy = result.ContentSecurityPolicy.Merge(other.ContentSecurityPolicy)

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
