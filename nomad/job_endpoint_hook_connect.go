package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/pkg/errors"
)

var (
	// connectSidecarResources returns the set of resources used by default for
	// the Consul Connect sidecar task
	connectSidecarResources = func() *structs.Resources {
		return &structs.Resources{
			CPU:      250,
			MemoryMB: 128,
		}
	}

	// connectDriverConfig is the driver configuration used by the injected
	// connect proxy sidecar task
	connectDriverConfig = func() map[string]interface{} {
		return map[string]interface{}{
			"image": "${meta.connect.sidecar_image}",
			"args": []interface{}{
				"-c", structs.EnvoyBootstrapPath,
				"-l", "${meta.connect.log_level}",
				"--disable-hot-restart",
			},
		}
	}

	// connectVersionConstraint is used when building the sidecar task to ensure
	// the proper Consul version is used that supports the necessary Connect
	// features. This includes bootstrapping envoy with a unix socket for Consul's
	// gRPC xDS API.
	connectVersionConstraint = func() *structs.Constraint {
		return &structs.Constraint{
			LTarget: "${attr.consul.version}",
			RTarget: ">= 1.6.0-beta1",
			Operand: structs.ConstraintSemver,
		}
	}
)

// jobConnectHook implements a job Mutating and Validating admission controller
type jobConnectHook struct{}

func (jobConnectHook) Name() string {
	return "connect"
}

func (jobConnectHook) Mutate(job *structs.Job) (_ *structs.Job, warnings []error, err error) {
	for _, g := range job.TaskGroups {
		// TG isn't validated yet, but validation
		// may depend on mutation results.
		// Do basic validation here and skip mutation,
		// so Validate can return a meaningful error
		// messages
		if len(g.Networks) == 0 {
			continue
		}

		if err := groupConnectHook(job, g); err != nil {
			return nil, nil, err
		}
	}

	return job, nil, nil
}

func (jobConnectHook) Validate(job *structs.Job) (warnings []error, err error) {
	for _, g := range job.TaskGroups {
		w, err := groupConnectValidate(g)
		if err != nil {
			return nil, err
		}
		if w != nil {
			warnings = append(warnings, w...)
		}
	}

	return warnings, nil
}

// getSidecarTaskForService looks for the sidecar task for a given service within
// a task group. If no sidecar task is found nil is returned
func getSidecarTaskForService(tg *structs.TaskGroup, svc string) *structs.Task {
	for _, t := range tg.Tasks {
		if isSidecarForService(t, svc) {
			return t
		}
	}
	return nil
}

func isSidecarForService(t *structs.Task, svc string) bool {
	return t.Kind == structs.TaskKind(fmt.Sprintf("%s:%s", structs.ConnectProxyPrefix, svc))
}

// getNamedTaskForNativeService retrieves the Task with the name specified in the
// group service definition. If the task name is empty and there is only one task
// in the group, infer the name from the only option.
func getNamedTaskForNativeService(tg *structs.TaskGroup, serviceName, taskName string) (*structs.Task, error) {
	if taskName == "" {
		if len(tg.Tasks) == 1 {
			return tg.Tasks[0], nil
		}
		return nil, errors.Errorf("task for Consul Connect Native service %s->%s is ambiguous and must be set", tg.Name, serviceName)
	}

	for _, t := range tg.Tasks {
		if t.Name == taskName {
			return t, nil
		}
	}
	return nil, errors.Errorf("task %s named by Consul Connect Native service %s->%s does not exist", taskName, tg.Name, serviceName)
}

// probably need to hack this up to look for checks on the service, and if they
// qualify, configure a port for envoy to use to expose their paths.
func groupConnectHook(job *structs.Job, g *structs.TaskGroup) error {
	for _, service := range g.Services {
		if service.Connect.HasSidecar() {
			// Check to see if the sidecar task already exists
			task := getSidecarTaskForService(g, service.Name)

			// If the task doesn't already exist, create a new one and add it to the job
			if task == nil {
				task = newConnectTask(service.Name)

				// If there happens to be a task defined with the same name
				// append an UUID fragment to the task name
				for _, t := range g.Tasks {
					if t.Name == task.Name {
						task.Name = task.Name + "-" + uuid.Generate()[:6]
						break
					}
				}
				g.Tasks = append(g.Tasks, task)
			}

			if service.Connect.SidecarTask != nil {
				service.Connect.SidecarTask.MergeIntoTask(task)
			}

			// Canonicalize task since this mutator runs after job canonicalization
			task.Canonicalize(job, g)

			makePort := func(label string) {
				// check that port hasn't already been defined before adding it to tg
				for _, p := range g.Networks[0].DynamicPorts {
					if p.Label == label {
						return
					}
				}
				g.Networks[0].DynamicPorts = append(g.Networks[0].DynamicPorts, structs.Port{
					Label: label,
					// -1 is a sentinel value to instruct the
					// scheduler to map the host's dynamic port to
					// the same port in the netns.
					To: -1,
				})
			}

			// create a port for the sidecar task's proxy port
			makePort(fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, service.Name))
		} else if service.Connect.IsNative() {
			// find the task backing this connect native service and set the kind
			nativeTaskName := service.TaskName
			if t, err := getNamedTaskForNativeService(g, service.Name, nativeTaskName); err != nil {
				return err
			} else {
				t.Kind = structs.NewTaskKind(structs.ConnectNativePrefix, service.Name)
				service.TaskName = t.Name // in case the task was inferred
			}
		}
	}

	// re-canonicalize group network since this hook runs after canonicalizaton
	g.Networks[0].Canonicalize()
	return nil
}

func newConnectTask(serviceName string) *structs.Task {
	task := &structs.Task{
		// Name is used in container name so must start with '[A-Za-z0-9]'
		Name:          fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, serviceName),
		Kind:          structs.NewTaskKind(structs.ConnectProxyPrefix, serviceName),
		Driver:        "docker",
		Config:        connectDriverConfig(),
		ShutdownDelay: 5 * time.Second,
		LogConfig: &structs.LogConfig{
			MaxFiles:      2,
			MaxFileSizeMB: 2,
		},
		Resources: connectSidecarResources(),
		Lifecycle: &structs.TaskLifecycleConfig{
			Hook:    structs.TaskLifecycleHookPrestart,
			Sidecar: true,
		},
		Constraints: structs.Constraints{
			connectVersionConstraint(),
		},
	}

	return task
}

func groupConnectValidate(g *structs.TaskGroup) (warnings []error, err error) {
	for _, s := range g.Services {
		if s.Connect.HasSidecar() {
			if err := groupConnectSidecarValidate(g); err != nil {
				return nil, err
			}
		} else if s.Connect.IsNative() {
			if err := groupConnectNativeValidate(g, s); err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

func groupConnectSidecarValidate(g *structs.TaskGroup) error {
	if n := len(g.Networks); n != 1 {
		return fmt.Errorf("Consul Connect sidecars require exactly 1 network, found %d in group %q", n, g.Name)
	}

	if g.Networks[0].Mode != "bridge" {
		return fmt.Errorf("Consul Connect sidecar requires bridge network, found %q in group %q", g.Networks[0].Mode, g.Name)
	}
	return nil
}

func groupConnectNativeValidate(g *structs.TaskGroup, s *structs.Service) error {
	// note that network mode is not enforced for connect native services

	if _, err := getNamedTaskForNativeService(g, s.Name, s.TaskName); err != nil {
		return err
	}
	return nil
}
