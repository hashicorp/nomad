// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package config

import "github.com/hashicorp/nomad/helper/pointer"

// SELinuxConfig configures how the client works with SELinux.
type SELinuxConfig struct {
	// LabelTaskSockets creates the unix sockets tasks connect to with the
	// container SELinux type, so tasks in confined containers can use them.
	LabelTaskSockets *bool `hcl:"label_task_sockets"`
}

// Copy returns a deep copy of the SELinux struct.
func (s *SELinuxConfig) Copy() *SELinuxConfig {
	if s == nil {
		return nil
	}
	return &SELinuxConfig{
		LabelTaskSockets: pointer.Copy(s.LabelTaskSockets),
	}
}

// Merge returns a new SELinux where non-nil fields in the argument have
// higher precedence.
func (s *SELinuxConfig) Merge(o *SELinuxConfig) *SELinuxConfig {
	switch {
	case s == nil:
		return o.Copy()
	case o == nil:
		return s.Copy()
	default:
		return &SELinuxConfig{
			LabelTaskSockets: pointer.Merge(s.LabelTaskSockets, o.LabelTaskSockets),
		}
	}
}
