package taskrunner

import "github.com/hashicorp/nomad/client/allocrunner/interfaces"

// Statically assert the logmon hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = (*logmonHook)(nil)
var _ interfaces.TaskStopHook = (*logmonHook)(nil)
