// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"

	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

// validateHook validates the task is able to be run.
type validateHook struct {
	config *config.Config
	logger log.Logger
}

func newValidateHook(config *config.Config, logger log.Logger) *validateHook {
	h := &validateHook{
		config: config,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*validateHook) Name() string {
	return "validate"
}

func (h *validateHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	if err := validateTask(req.Task, req.TaskEnv, h.config); err != nil {
		return err
	}

	resp.Done = true
	return nil
}

func validateTask(task *structs.Task, taskEnv *taskenv.TaskEnv, conf *config.Config) error {
	var mErr multierror.Error

	// Validate the user
	// COMPAT(1.0) uses inclusive language. blacklist is kept for backward compatilibity.
	unallowedUsers := conf.ReadStringListAlternativeToMapDefault(
		[]string{"user.denylist", "user.blacklist"},
		config.DefaultUserDenylist,
	)
	checkDrivers := conf.ReadStringListToMapDefault("user.checked_drivers", config.DefaultUserCheckedDrivers)
	if _, driverMatch := checkDrivers[task.Driver]; driverMatch {
		if _, unallowed := unallowedUsers[task.User]; unallowed {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("running as user %q is disallowed", task.User))
		}
	}

	// Validate the Service names once they're interpolated
	for _, service := range task.Services {
		name := taskEnv.ReplaceEnv(service.Name)
		if err := service.ValidateName(name); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("service (%s) failed validation: %v", name, err))
		}
	}

	if len(mErr.Errors) == 1 {
		return mErr.Errors[0]
	}
	return mErr.ErrorOrNil()
}
