// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import "github.com/hashicorp/nomad/client/allocrunner/interfaces"

// Statically assert the stats hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = (*vaultHook)(nil)
var _ interfaces.TaskStopHook = (*vaultHook)(nil)
var _ interfaces.ShutdownHook = (*vaultHook)(nil)
