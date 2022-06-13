package taskrunner

import (
	"context"
	"fmt"
	"net/url"

	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"go.opentelemetry.io/otel/baggage"
)

const envKeyOtelResourceAttrs = "OTEL_RESOURCE_ATTRIBUTES"

type otelHookConfig struct {
	logger log.Logger
	alloc  *structs.Allocation
	node   *structs.Node
}

type otelHook struct {
	alloc  *structs.Allocation
	node   *structs.Node
	logger log.Logger
}

func newOtelHook(config *otelHookConfig) *otelHook {
	hook := &otelHook{
		alloc: config.alloc,
		node:  config.node,
	}
	hook.logger = config.logger.Named(hook.Name()).
		With("alloc_id", config.alloc.ID)

	return hook
}

func (h *otelHook) Name() string {
	return "otel"
}

func (h *otelHook) Prestart(_ context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	logger := h.logger.With("task", req.Task.Name)

	resourceAttrsEnv, ok := req.TaskEnv.EnvMap[envKeyOtelResourceAttrs]
	if ok && resourceAttrsEnv == "" {
		logger.Debug("skipping OTEL_RESOURCE_ATTRIBUTES environment variable")
		return nil
	}

	resourceAttrs, err := generateBaggage(h.alloc, req.Task, h.node)
	if err != nil {
		logger.Warn("failed to generate OTEL_RESOURCE_ATTRIBUTES environment variable", "error", err)
		return nil
	}

	if resourceAttrsEnv != "" {
		logger.Debug("merging existing OTEL_RESOURCE_ATTRIBUTES environment variable values", "attrs", resourceAttrsEnv)

		taskBaggage, err := baggage.Parse(resourceAttrsEnv)
		if err != nil {
			logger.Warn("failed to parse task environment variable OTEL_RESOURCE_ATTRIBUTES as baggage",
				"otel_resource_attributes", resourceAttrsEnv, "error", err)
		} else {
			for _, m := range taskBaggage.Members() {
				k, v := m.Key(), m.Value()
				logger.Trace("found member", "key", k, "value", v)

				// TODO(luiz): don't create new member once baggage.Members()
				// returns values with `hasData` set to `true`.
				// https://github.com/open-telemetry/opentelemetry-go/issues/3164
				member, err := baggage.NewMember(k, v)
				if err != nil {
					logger.Warn("failed to create new baggage member", "key", k, "value", v, "error", err)
					continue
				}

				resourceAttrs, err = resourceAttrs.SetMember(member)
				if err != nil {
					logger.Warn("failed to set new baggage member", "key", k, "value", v, "error", err)
					continue
				}
			}
		}
	}

	// TODO(luiz): remove decode step once the Otel SDK handles it internally.
	// https://github.com/open-telemetry/opentelemetry-go/pull/2963
	attrs, err := url.QueryUnescape(resourceAttrs.String())
	if err != nil {
		attrs = resourceAttrs.String()
	}
	resp.Env = map[string]string{
		envKeyOtelResourceAttrs: attrs,
	}
	return nil
}

func generateBaggage(alloc *structs.Allocation, task *structs.Task, node *structs.Node) (baggage.Baggage, error) {
	var mErr *multierror.Error
	job := alloc.Job
	members := []baggage.Member{
		newMember("nomad.alloc.createTime", fmt.Sprintf("%v", alloc.CreateTime), mErr),
		newMember("nomad.alloc.id", alloc.ID, mErr),
		newMember("nomad.alloc.name", alloc.Name, mErr),
		newMember("nomad.eval.id", alloc.EvalID, mErr),
		newMember("nomad.group.name", alloc.TaskGroup, mErr),
		newMember("nomad.job.id", job.ID, mErr),
		newMember("nomad.job.name", job.Name, mErr),
		newMember("nomad.job.region", job.Region, mErr),
		newMember("nomad.job.type", job.Type, mErr),
		newMember("nomad.namespace", alloc.Namespace, mErr),
		newMember("nomad.node.id", node.ID, mErr),
		newMember("nomad.node.name", node.Name, mErr),
		newMember("nomad.node.datacenter", node.Datacenter, mErr),
		newMember("nomad.task.name", task.Name, mErr),
		newMember("nomad.task.driver", task.Driver, mErr),
	}
	if job.ParentID != "" {
		members = append(members, newMember("nomad.job.parentId", job.ParentID, mErr))
	}
	if node.NodeClass != "" {
		members = append(members, newMember("nomad.node.class", node.NodeClass, mErr))
	}
	if err := mErr.ErrorOrNil(); err != nil {
		return baggage.Baggage{}, err
	}

	b, err := baggage.New(members...)
	if err != nil {
		_ = multierror.Append(mErr, err)
	}
	return b, mErr.ErrorOrNil()
}

func newMember(key, value string, mErr *multierror.Error) baggage.Member {
	m, err := baggage.NewMember(key, value)
	if err != nil {
		_ = multierror.Append(mErr, err)
	}
	return m
}
