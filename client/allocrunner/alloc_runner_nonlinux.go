// +build !linux

package allocrunner

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
)

// noop for non linux clients
func (ar *allocRunner) initPlatformRunnerHooks(hookLogger hclog.Logger) ([]interfaces.RunnerHook, error) {
	return nil, nil
}
