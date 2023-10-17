// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package taskrunner

import "github.com/hashicorp/nomad/client/allocrunner/interfaces"

var _ interfaces.TaskPrestartHook = (*identityHook)(nil)

// See task_runner_test.go:TestTaskRunner_IdentityHook
