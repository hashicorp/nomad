// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

type RootlessConfig struct {
	MounterSocket string `hcl:"mounter_socket"`
}

func (r *RootlessConfig) Copy() *RootlessConfig {
	if r == nil {
		return nil
	}
	nr := *r
	return &nr
}

func (r *RootlessConfig) Merge(o *RootlessConfig) *RootlessConfig {
	if r == nil {
		return o.Copy()
	}
	result := r.Copy()
	if o == nil {
		return result
	}
	if o.MounterSocket != "" {
		result.MounterSocket = o.MounterSocket
	}
	return result
}
