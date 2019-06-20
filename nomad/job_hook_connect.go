package nomad

import "github.com/hashicorp/nomad/nomad/structs"

type jobConnectHook struct{}

func (jobConnectHook) Name() string {
	return "connect"
}

func (jobConnectHook) Mutate(job *structs.Job) (_ *structs.Job, warnings []error, err error) {
	for _, g := range job.TaskGroups {
		groupConnectHook(g)
	}

	return job, nil, nil
}

func groupConnectHook(g *structs.TaskGroup) {
	for _, service := range g.Services {
		if service.Connect.HasSidecar() {
			task := newConnectTask(service)
			g.Tasks = append(g.Tasks, task)
			return
		}
	}
}

//TODO(schmichael) create a sidecar proxy task from a Connect-enabled Service
//TODO user templating/configuration
//TODO restart/reschedule stanza
func newConnectTask(service *structs.Service) *structs.Task {
	return &structs.Task{
		Name:   "_envoy",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "nomad-consul-envoy:0.10.0",
			"command": "envoy",
			"args":    []string{"-c", "local/bootstrap.json"},
		},
		Order: -1,
		LogConfig: &structs.LogConfig{
			MaxFiles:      2,
			MaxFileSizeMB: 2,
		},
		Resources: structs.DefaultResources(),
	}
}
