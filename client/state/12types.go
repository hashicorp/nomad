// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import "github.com/hashicorp/nomad/client/dynamicplugins"

// RegistryState12 is the dynamic plugin registry state persisted
// before 1.3.0.
type RegistryState12 struct {
	Plugins map[string]map[string]*dynamicplugins.PluginInfo
}
