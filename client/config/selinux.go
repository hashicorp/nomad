// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package config

import sconfig "github.com/hashicorp/nomad/nomad/structs/config"

// SELinuxConfig configures how the client works with SELinux.
type SELinuxConfig struct {
	// LabelTaskSockets creates the unix sockets tasks connect to with the
	// container SELinux type, so tasks in confined containers can use them.
	LabelTaskSockets bool
}

func SELinuxConfigFromAgent(c *sconfig.SELinuxConfig) *SELinuxConfig {
	if c == nil || c.LabelTaskSockets == nil {
		return &SELinuxConfig{}
	}
	return &SELinuxConfig{
		LabelTaskSockets: *c.LabelTaskSockets,
	}
}

func (s *SELinuxConfig) Copy() *SELinuxConfig {
	if s == nil {
		return nil
	}
	return &SELinuxConfig{
		LabelTaskSockets: s.LabelTaskSockets,
	}
}

// TaskSocketsLabeled reports whether task sockets are created with the
// container SELinux type.
func (s *SELinuxConfig) TaskSocketsLabeled() bool {
	return s != nil && s.LabelTaskSockets
}
