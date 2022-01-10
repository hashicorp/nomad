package nomad

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/pkg/errors"
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
	// TODO: Should this not get checked by another validator before?
	if ns == nil {
		return nil, errors.Errorf("requested namespace %s does not exist", job.Namespace)
	}

	for _, tg := range job.TaskGroups {
		for _, t := range tg.Tasks {
			if !taskValidateDriver(t, ns) {
				return nil, errors.Errorf(
					"used task driver '%s' in %s[%s] is not allowed in namespace %s",
					t.Driver, tg.Name, t.Name, ns.Name)
			}
		}
	}
	return nil, nil
}

func taskValidateDriver(task *structs.Task, ns *structs.Namespace) bool {
	if ns.Capabilities == nil || len(ns.Capabilities.EnabledTaskDrivers) == 0 {
		return true
	}
	for _, d := range ns.Capabilities.EnabledTaskDrivers {
		if task.Driver == d {
			return true
		}
	}
	return false
}
