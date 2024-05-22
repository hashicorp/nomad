// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package allocrunner

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (ar *allocRunner) SetTaskPauseState(string, structs.TaskScheduleState) error {
	return fmt.Errorf("Enterprise only")
}

func (ar *allocRunner) GetTaskPauseState(taskName string) (structs.TaskScheduleState, error) {
	return "", fmt.Errorf("Enterprise only")
}
