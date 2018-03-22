// +build pro ent

package agent

import "github.com/hashicorp/nomad/helper"

// DefaultEntConfig allows configuring enterprise only default configuration
// values.
func DefaultEntConfig() *Config {
	return &Config{
		DisableUpdateCheck: helper.BoolToPtr(true),
	}
}
