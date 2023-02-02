package taskrunner

import "github.com/hashicorp/nomad/client/allocrunner/interfaces"

var _ interfaces.TaskPrestartHook = (*identityHook)(nil)
var _ interfaces.TaskUpdateHook = (*identityHook)(nil)

// See task_runner_test.go:TestTaskRunner_IdentityHook
