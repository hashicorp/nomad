package taskrunner

import (
	"context"
	"fmt"
	"strings"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"go.opentelemetry.io/otel/baggage"
)

var _ interfaces.TaskPrestartHook = &otelHook{}

type otelHook struct {
	alloc  *structs.Allocation
	task   *structs.Task
	node   *structs.Node
	logger log.Logger
}

func newOtelHook(alloc *structs.Allocation, task *structs.Task, node *structs.Node, logger log.Logger) *otelHook {
	hook := &otelHook{
		alloc: alloc,
		task:  task,
		node:  node,
	}
	hook.logger = logger.Named(hook.Name()).
		With("alloc_id", alloc.ID).
		With("task", task.Name)
	return hook
}

func (h *otelHook) Name() string {
	return "otel"
}

func (h *otelHook) Prestart(_ context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	const resourceAttrsKey = "OTEL_RESOURCE_ATTRIBUTES"

	job := h.alloc.Job
	members := []baggage.Member{
		h.newMember("nomad.alloc.id", h.alloc.ID),
		h.newMember("nomad.alloc.name", h.alloc.Name),
		h.newMember("nomad.alloc.createTime", fmt.Sprintf("%v", h.alloc.CreateTime)),
		h.newMember("nomad.eval.id", h.alloc.EvalID),
		h.newMember("nomad.group.name", h.alloc.TaskGroup),
		h.newMember("nomad.job.id", job.ID),
		h.newMember("nomad.job.name", job.Name),
		h.newMember("nomad.job.parentId", job.ParentID),
		h.newMember("nomad.job.type", job.Type),
		h.newMember("nomad.namespace", h.alloc.Namespace),
		h.newMember("nomad.node.id", h.node.ID),
		h.newMember("nomad.node.name", h.node.Name),
		h.newMember("nomad.node.datacenter", h.node.Datacenter),
		h.newMember("nomad.node.class", h.node.NodeClass),
		h.newMember("nomad.node.address", h.node.HTTPAddr),
		h.newMember("nomad.region", job.Region),
		h.newMember("nomad.task.name", h.task.Name),
		h.newMember("nomad.task.driver", h.task.Driver),
	}

	switch h.task.Driver {
	case "docker":
		members = append(members, h.newMember("container.runtime", "docker"))

		image := h.task.Config["image"].(string)
		parts := strings.SplitN(image, ":", 2)
		members = append(members, h.newMember("container.image.name", parts[0]))
		if len(parts) > 1 {
			members = append(members, h.newMember("container.image.tag", parts[1]))
		}
	}

	resourceAttrs, _ := baggage.New(members...)

	// TODO(luiz): fix merge of task env.
	if taskAttrs, ok := h.task.Env[resourceAttrsKey]; ok {
		taskBaggage, err := baggage.Parse(taskAttrs)
		if err != nil {
			h.logger.Warn("failed to parse task baggage", "error", err)
		}
		for _, m := range taskBaggage.Members() {
			resourceAttrs, err = resourceAttrs.SetMember(m)
			if err != nil {
				h.logger.Warn("failed to set new baggage member", "error", err)
			}
		}
	}

	h.logger.Debug("task environment", resourceAttrsKey, resourceAttrs)
	resp.Env = map[string]string{
		resourceAttrsKey: resourceAttrs.String(),
	}
	return nil
}

func (h *otelHook) newMember(key, value string) baggage.Member {
	m, err := baggage.NewMember(key, value)
	if err != nil {
		h.logger.Warn("failed to create new baggage member",
			"key", key, "value", value, "error", err)
	}
	return m
}
