package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// connectSidecarResources is the set of resources used by default for the
	// Consul Connect sidecar task
	connectSidecarResources = structs.DefaultResources()

	// connectVersionConstraint is used when building the sidecar task to ensure
	// the proper Consul version is used that supports the nessicary Connect
	// features. This includes bootstraping envoy with a unix socket for Consul's
	// grpc xDS api.
	connectVersionConstraint = &structs.Constraint{
		LTarget: "${attr.consul.version}",
		RTarget: ">= 1.6.1",
		Operand: "version",
	}
)

// jobConnectHook implements a job  Mutating and Validating admission controller
type jobConnectHook struct{}

func (jobConnectHook) Name() string {
	return "connect"
}

func (jobConnectHook) Mutate(job *structs.Job) (_ *structs.Job, warnings []error, err error) {
	for _, g := range job.TaskGroups {
		if err := groupConnectHook(g); err != nil {
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

func groupConnectHook(g *structs.TaskGroup) error {
	for _, service := range g.Services {
		if service.Connect.HasSidecar() {
			// Check to see if the sidecar task already exists
			task := getSidecarTaskForService(g, service.Name)

			// If the task doesn't already exist, create a new one and add it to the job
			if task == nil {
				task = newConnectTask(service)
				g.Tasks = append(g.Tasks, task)
			}

			//TODO merge in sidecar_task overrides

			port := structs.Port{
				Label: fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, service.Name),

				// -1 is a sentinel value to instruct the
				// scheduler to map the host's dynamic port to
				// the same port in the netns.
				To: -1,
			}
			g.Networks[0].DynamicPorts = append(g.Networks[0].DynamicPorts, port)
			return nil
		}
	}
	return nil
}

func newConnectTask(service *structs.Service) *structs.Task {
	task := &structs.Task{
		Name:   fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, service.Name), // used in container name so must start with '[A-Za-z0-9]'
		Kind:   structs.TaskKind(fmt.Sprintf("%s:%s", structs.ConnectProxyPrefix, service.Name)),
		Driver: "docker",
		Config: map[string]interface{}{
			"image": "${meta.connect.sidecar_image}",
			"args": []string{
				"-c", "${NOMAD_TASK_DIR}/bootstrap.json",
				"-l", "${meta.connect.log_level}",
			},
		},
		ShutdownDelay: time.Second * 5,
		LogConfig: &structs.LogConfig{
			MaxFiles:      2,
			MaxFileSizeMB: 2,
		},
		Resources: connectSidecarResources,
		Constraints: structs.Constraints{
			connectVersionConstraint,
		},
	}

	return task
}

func groupConnectValidate(g *structs.TaskGroup) (warnings []error, err error) {
	for _, s := range g.Services {
		if s.Connect.HasSidecar() {
			if n := len(g.Networks); n != 1 {
				return nil, fmt.Errorf("Consul Connect sidecars require exactly 1 network, found %d in group %q", n, g.Name)
			}

			if g.Networks[0].Mode != "bridge" {
				return nil, fmt.Errorf("Consul Connect sidecar requires bridge network, found %q in group %q", g.Networks[0].Mode, g.Name)
			}

			// Stopping loop, only need to do the validation once
			break
		}
	}

	return nil, nil
}
