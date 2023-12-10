// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

type jobNamespaceConstraintCheckHook struct {
	srv *Server
}

func (jobNamespaceConstraintCheckHook) Name() string {
	return "namespace-constraint-check"
}

func (c jobNamespaceConstraintCheckHook) Validate(job *structs.Job) (warnings []error, err error) {
	// This was validated before and matches the WriteRequest namespace
	ns, err := c.srv.State().NamespaceByName(nil, job.Namespace)
	if err != nil {
		return nil, err
	}
	if ns == nil {
		return nil, fmt.Errorf("job %q is in nonexistent namespace %q", job.ID, job.Namespace)
	}

	var disallowedDrivers []string
	for _, tg := range job.TaskGroups {
		for _, t := range tg.Tasks {
			if !taskValidateDriver(t, ns) {
				disallowedDrivers = append(disallowedDrivers, t.Driver)
			}
		}
	}
	if len(disallowedDrivers) > 0 {
		if len(disallowedDrivers) == 1 {
			return nil, fmt.Errorf(
				"used task driver %q is not allowed in namespace %q", disallowedDrivers[0], ns.Name,
			)

		} else {
			return nil, fmt.Errorf(
				"used task drivers %q are not allowed in namespace %q", disallowedDrivers, ns.Name,
			)
		}
	}
	return nil, nil
}

func taskValidateDriver(task *structs.Task, ns *structs.Namespace) bool {
	if ns.Capabilities == nil {
		return true
	}
	allow := len(ns.Capabilities.EnabledTaskDrivers) == 0
	for _, d := range ns.Capabilities.EnabledTaskDrivers {
		if task.Driver == d {
			allow = true
			break
		}
	}
	for _, d := range ns.Capabilities.DisabledTaskDrivers {
		if task.Driver == d {
			allow = false
			break
		}
	}
	return allow
}
