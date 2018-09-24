package taskrunner

import "github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"

// Statically assert the logmon hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = (*logmonHook)(nil)
var _ interfaces.TaskExitedHook = (*logmonHook)(nil)
