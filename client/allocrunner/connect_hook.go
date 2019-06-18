package allocrunner

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

type connectHook struct {
	alloc *structs.Allocation

	logger hclog.Logger
}

func newConnectHook(logger hclog.Logger, alloc *structs.Allocation) *connectHook {
	return &connectHook{
		alloc:  alloc,
		logger: logger,
	}
}

func (h *connectHook) Name() string {
	return "connect"
}

func (h *connectHook) Init() error {
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)

	//TODO(schmichael) loop over all services in all places? not just tg->services?
	//TODO(schmichael) handle multiple sidecars
	for _, service := range tg.Services {
		if service.Connect.HasSidecar() {
			injectConnectTask(h.alloc, service)
			return nil
		}
	}

	return nil
}

//TODO(schmichael) injectConnectTask converts a Connect-enabled Service to a task
func injectConnectTask(alloc *structs.Allocation, service *structs.Service) {
	task := &structs.Task{
		Name:   "consul-connect",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "nomad-consul-envoy",
			"command": "consul-envoy",
			"args":    []string{"-sidecar-for", service.Name},
		},
		//TODO(schmichael) resources?
		Order: -1,
	}

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	task.Canonicalize(alloc.Job, tg)
	tg.Tasks = append(tg.Tasks, task)
}
