// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package nomad

// establishEnterpriseLeadership is a no-op on OSS.
func (s *Server) establishEnterpriseLeadership(stopCh chan struct{}) error {
	return nil
}

// revokeEnterpriseLeadership is a no-op on OSS>
func (s *Server) revokeEnterpriseLeadership() error {
	return nil
}
