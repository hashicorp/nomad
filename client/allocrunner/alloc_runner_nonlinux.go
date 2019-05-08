// +build !linux

package allocrunner

import hclog "github.com/hashicorp/go-hclog"

// noop for non linux clients
func (ar *allocRunner) initPlatformRunnerHooks(hookLogger hclog.Logger) {}
