package taskrunner

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/mock"
	"go.opentelemetry.io/otel/baggage"

	"github.com/shoenig/test/must"
)

// Statically assert the otel hook implements the expected interfaces
var _ interfaces.TaskPrestartHook = &otelHook{}

func TestTaskRunner_OtelHook(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                  string
		taskEnv               map[string]string
		expectNomadAttrs      bool
		expectAdditionalAttrs map[string]string
	}{
		{
			name:             "tasks have otel resource attributes env var",
			expectNomadAttrs: true,
		},
		{
			name: "disable otel resource attributes env var",
			taskEnv: map[string]string{
				envKeyOtelResourceAttrs: "",
			},
			expectNomadAttrs: false,
		},
		{
			name: "merge otel resource attributes env var",
			taskEnv: map[string]string{
				envKeyOtelResourceAttrs: "test=true",
			},
			expectNomadAttrs: true,
			expectAdditionalAttrs: map[string]string{
				"test": "true",
			},
		},
		{
			name: "invalid values are ignored",
			taskEnv: map[string]string{
				envKeyOtelResourceAttrs: "not-valid",
			},
			expectNomadAttrs: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alloc := mock.Alloc()
			node := mock.Node()
			task := mock.Job().TaskGroups[0].Tasks[0]

			otelHook := newOtelHook(&otelHookConfig{
				logger: hclog.NewNullLogger(),
				alloc:  alloc,
				node:   node,
			})

			// Setup task environment with addition test values.
			builder := taskenv.NewBuilder(node, alloc, task, "global")
			taskEnv := builder.Build()
			for k, v := range tc.taskEnv {
				taskEnv.EnvMap[k] = v
			}

			// Run hook.
			req := &interfaces.TaskPrestartRequest{
				TaskEnv: taskEnv,
				TaskDir: &allocdir.TaskDir{Dir: os.TempDir()},
				Task:    task,
			}
			resp := interfaces.TaskPrestartResponse{}
			err := otelHook.Prestart(context.Background(), req, &resp)
			must.NoError(t, err)

			// Read and parse resulting OTEL_RESOURCE_ATTRIBUTES env var.
			got := resp.Env[envKeyOtelResourceAttrs]
			b, err := baggage.Parse(got)
			must.NoError(t, err)

			if tc.expectNomadAttrs {
				must.Eq(t, b.Member("nomad.alloc.id").Value(), alloc.ID)
				must.Eq(t, b.Member("nomad.alloc.name").Value(), alloc.Name)
				must.Eq(t, b.Member("nomad.alloc.createTime").Value(), fmt.Sprintf("%v", alloc.CreateTime))
				must.Eq(t, b.Member("nomad.eval.id").Value(), alloc.EvalID)
				must.Eq(t, b.Member("nomad.job.id").Value(), alloc.Job.ID)
				must.Eq(t, b.Member("nomad.job.name").Value(), alloc.Job.Name)
				must.Eq(t, b.Member("nomad.job.region").Value(), alloc.Job.Region)
				must.Eq(t, b.Member("nomad.job.type").Value(), alloc.Job.Type)
				must.Eq(t, b.Member("nomad.namespace").Value(), alloc.Namespace)
				must.Eq(t, b.Member("nomad.node.id").Value(), node.ID)
				must.Eq(t, b.Member("nomad.node.name").Value(), node.Name)
				must.Eq(t, b.Member("nomad.node.datacenter").Value(), node.Datacenter)
				must.Eq(t, b.Member("nomad.task.name").Value(), task.Name)
				must.Eq(t, b.Member("nomad.task.driver").Value(), task.Driver)

				if alloc.Job.ParentID != "" {
					must.Eq(t, b.Member("nomad.job.parentId").Value(), alloc.Job.ParentID)
				} else {
					must.Eq(t, b.Member("nomad.job.parentId"), baggage.Member{})
				}

				if node.NodeClass != "" {
					must.Eq(t, b.Member("nomad.node.class").Value(), node.NodeClass)
				} else {
					must.Eq(t, b.Member("nomad.node.class"), baggage.Member{})
				}
			} else {
				must.Eq(t, got, "")
			}

			if len(tc.expectAdditionalAttrs) > 0 {
				for k, v := range tc.expectAdditionalAttrs {
					must.Eq(t, b.Member(k).Value(), v)
				}
			} else {
				for _, m := range b.Members() {
					// If not additional values are expected, all attributes
					// must be related to Nomad.
					must.StrContains(t, m.Key(), "nomad")
				}
			}
		})
	}
}
