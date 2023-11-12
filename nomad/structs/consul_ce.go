// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package structs

func (c *Consul) GetNamespace() string {
	return ""
}

// GetConsulClusterName gets the Consul cluster for this task. Only a single
// default cluster is supported in Nomad CE.
func (t *Task) GetConsulClusterName(_ *TaskGroup) string {
	return ConsulDefaultCluster
}

// GetConsulClusterName gets the Consul cluster for this service. Only a single
// default cluster is supported in Nomad CE.
func (s *Service) GetConsulClusterName(_ *TaskGroup) string {
	return ConsulDefaultCluster
}
