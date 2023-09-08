// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// FailHook is designed to fail for testing purposes,
// so should never be included in a release.
//go:build !release

package allocrunner

import (
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/hcl/v2/hclsimple"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
)

var ErrFailHookError = errors.New("failed successfully")

func NewFailHook(l hclog.Logger, name string) *FailHook {
	return &FailHook{
		name:   name,
		logger: l.Named(name),
	}
}

type FailHook struct {
	name   string
	logger hclog.Logger
	Fail   struct {
		Prerun         bool `hcl:"prerun,optional"`
		PreKill        bool `hcl:"prekill,optional"`
		Postrun        bool `hcl:"postrun,optional"`
		Destroy        bool `hcl:"destroy,optional"`
		Update         bool `hcl:"update,optional"`
		PreTaskRestart bool `hcl:"pretaskrestart,optional"`
		Shutdown       bool `hcl:"shutdown,optional"`
	}
}

func (h *FailHook) Name() string {
	return h.name
}

func (h *FailHook) LoadConfig(path string) *FailHook {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		h.logger.Error("couldn't load config", "error", err)
		return h
	}
	if err := hclsimple.DecodeFile(path, nil, &h.Fail); err != nil {
		h.logger.Error("error parsing config", "path", path, "error", err)
	}
	return h
}

var _ interfaces.RunnerPrerunHook = &FailHook{}

func (h *FailHook) Prerun() error {
	if h.Fail.Prerun {
		return fmt.Errorf("prerun %w", ErrFailHookError)
	}
	return nil
}

var _ interfaces.RunnerPreKillHook = &FailHook{}

func (h *FailHook) PreKill() {
	if h.Fail.PreKill {
		h.logger.Error("prekill", "error", ErrFailHookError)
	}
}

var _ interfaces.RunnerPostrunHook = &FailHook{}

func (h *FailHook) Postrun() error {
	if h.Fail.Postrun {
		return fmt.Errorf("postrun %w", ErrFailHookError)
	}
	return nil
}

var _ interfaces.RunnerDestroyHook = &FailHook{}

func (h *FailHook) Destroy() error {
	if h.Fail.Destroy {
		return fmt.Errorf("destroy %w", ErrFailHookError)
	}
	return nil
}

var _ interfaces.RunnerUpdateHook = &FailHook{}

func (h *FailHook) Update(request *interfaces.RunnerUpdateRequest) error {
	if h.Fail.Update {
		return fmt.Errorf("update %w", ErrFailHookError)
	}
	return nil
}

var _ interfaces.RunnerTaskRestartHook = &FailHook{}

func (h *FailHook) PreTaskRestart() error {
	if h.Fail.PreTaskRestart {
		return fmt.Errorf("destroy %w", ErrFailHookError)
	}
	return nil
}

var _ interfaces.ShutdownHook = &FailHook{}

func (h *FailHook) Shutdown() {
	if h.Fail.Shutdown {
		h.logger.Error("shutdown", "error", ErrFailHookError)
	}
}
