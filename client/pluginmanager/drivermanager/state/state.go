// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import pstructs "github.com/hashicorp/nomad/plugins/shared/structs"

// PluginState is used to store the driver manager's state across restarts of the
// agent
type PluginState struct {
	// ReattachConfigs are the set of reattach configs for plugins launched by
	// the driver manager
	ReattachConfigs map[string]*pstructs.ReattachConfig
}
