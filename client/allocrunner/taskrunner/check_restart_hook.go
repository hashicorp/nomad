package taskrunner

import (
	"context"
	"fmt"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

type hookCheck struct {
	// providerType defines the backend service that is checking
	// the service. This is currently either Nomad or Consul.
	providerType string

	// providerNS is the providers namespace that the service is
	// registered. When a provider implements namespaces (i.e. Consul),
	// Nomad runs a single check watcher per namespace.
	providerNS string

	// checkID is the ID of the check used to register a check_restart watch
	checkID string

	// check is the actual Nomad service check configuration
	check *structs.ServiceCheck
}

// The checkRestartHook is responsible for registering/deregistering _both_ group and task
// check_start blocks with the appropriate CheckWatcher. This is a standalone hook and not part
// of the service hook because restarting checks is task specific, even though check_restart
// can be defined at the group level. Therefore this task will look at both TG and task services.
type checkRestartHook struct {
	checks   []*hookCheck
	handler  *wrapper.HandlerWrapper
	wr       serviceregistration.WorkloadRestarter
	allocID  string
	taskName string
	taskEnv  *taskenv.TaskEnv
}

func newCheckRestartHook(alloc *structs.Allocation, task *structs.Task, handler *wrapper.HandlerWrapper, taskEnv *taskenv.TaskEnv) *checkRestartHook {
	var checks []*hookCheck

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	for _, s := range taskenv.InterpolateServices(taskEnv, tg.Services) {
		for _, c := range s.Checks {
			if c.TriggersRestarts() && c.TaskName == task.Name {
				checks = append(checks, &hookCheck{
					providerType: s.Provider,
					providerNS:   s.Cluster,
					check:        c,
					// TODO: does this work for Nomad?
					checkID: checkID(alloc.ID, tg.Name, fmt.Sprintf("group-%s", tg.Name), s.Provider, c, s),
				})
			}
		}
	}

	for _, s := range taskenv.InterpolateServices(taskEnv, task.Services) {
		for _, c := range s.Checks {
			if c.TriggersRestarts() {
				checks = append(checks, &hookCheck{
					providerType: s.Provider,
					providerNS:   s.Cluster,
					check:        c,
					// TODO: does this work for Nomad?
					checkID: checkID(alloc.ID, tg.Name, task.Name, s.Provider, c, s),
				})
			}
		}
	}

	return &checkRestartHook{
		checks:   checks,
		handler:  handler,
		allocID:  alloc.ID,
		taskName: task.Name,
	}
}

func (h *checkRestartHook) Name() string {
	return "check_restart"
}

func (h *checkRestartHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, _ *interfaces.TaskPrestartResponse) error {
	for _, c := range h.checks {
		watcher := h.handler.CheckWatcher(c.providerType, c.providerNS)
		watcher.Watch(c.checkID, c.check, h.wr)
	}
	return nil
}

func (h *checkRestartHook) Exited(ctx context.Context, req *interfaces.TaskExitedRequest, resp *interfaces.TaskExitedResponse) error {
	for _, c := range h.checks {
		watcher := h.handler.CheckWatcher(c.providerType, c.providerNS)
		watcher.Unwatch(c.checkID)
	}
	return nil
}

func (h *checkRestartHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	for _, c := range h.checks {
		watcher := h.handler.CheckWatcher(c.providerType, c.providerNS)
		watcher.Unwatch(c.checkID)
	}
	return nil
}

// checkID returns a provider specific checkID for the workload. Unfortunately nomad and consul use different
// methods for creating checkID's so these are quite different. Consul distinguishes between group and task
// checkID's, but Nomad seems to just always use the task group name?
func checkID(allocID, tg, task, ptype string, check *structs.ServiceCheck, service *structs.Service) string {
	switch ptype {
	case "nomad":
		return string(structs.NomadCheckID(allocID, tg, check))
	case "consul":
		return consul.MakeCheckID(serviceregistration.MakeAllocServiceID(allocID, task, service), check)
	default:
		return ""
	}
}
