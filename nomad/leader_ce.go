// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import "github.com/hashicorp/nomad/v2/nomad/structs"

// establishEnterpriseLeadership is a no-op on OSS.
func (s *Server) establishEnterpriseLeadership(stopCh chan struct{}, clusterMD structs.ClusterMetadata) error {
	return nil
}

// revokeEnterpriseLeadership is a no-op on OSS>
func (s *Server) revokeEnterpriseLeadership() error {
	return nil
}
