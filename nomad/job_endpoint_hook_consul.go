// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

// jobConsulHook is a job registration admission controller for Consul
// configuration in Consul, Service, and Template blocks
type jobConsulHook struct {
	srv *Server
}

func (jobConsulHook) Name() string {
	return "consul"
}
