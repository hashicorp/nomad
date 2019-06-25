package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

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

func groupConnectHook(g *structs.TaskGroup) error {
	for _, service := range g.Services {
		if service.Connect.HasSidecar() {
			task := newConnectTask(service)
			g.Tasks = append(g.Tasks, task)

			//TODO(schmichael) FIXME
			if n := len(g.Networks); n != 1 {
				return fmt.Errorf("Consul Connect sidecars require exactly 1 network, found %d in group %q", n, g.Name)
			}

			if g.Networks[0].Mode != "bridge" {
				//TODO(schmichael) FIXME test
				return fmt.Errorf("Consul Connect sidecar requires bridge network, found %q in group %q", g.Networks[0].Mode, g.Name)
			}

			port := structs.Port{
				Label: "nomad_envoy",
				To:    -1, //FIXME(schmichael) hack to make a dynamic port match a To port
			}
			g.Networks[0].DynamicPorts = append(g.Networks[0].DynamicPorts, port)
			return nil

			/*
				//TODO(schmichael) ugly hack to get a free port inside the network
				used := map[int]struct{}{}
				for _, p := range g.Networks[0].DynamicPorts {
					used[p.To] = struct{}{}
				}
				for _, p := range g.Networks[0].ReservedPorts {
					used[p.To] = struct{}{}
				}
				for toPort := 12001; toPort < 65536; toPort++ {
					if _, ok := used[toPort]; ok {
						continue
					}
					port.To = toPort
					g.Networks[0].DynamicPorts = append(g.Networks[0].DynamicPorts, port)
					return nil
				}
				return fmt.Errorf("no unused To ports")
			*/
		}
	}
	return nil
}

//TODO(schmichael) create a sidecar proxy task from a Connect-enabled Service
//TODO user templating/configuration
//TODO restart/reschedule stanza
func newConnectTask(service *structs.Service) *structs.Task {
	task := &structs.Task{
		Name:   "nomad_envoy", // used in container name so must start with '[A-Za-z0-9]'
		Driver: "docker",
		Config: map[string]interface{}{
			"image": "envoyproxy/envoy:v1.10.0", //TODO(schmichael) TBD what image to use
			"args": []string{
				"-c", "local/bootstrap.json",
				//"-l", "debug", //TODO(schmichael) add a way to enable
			},
		},
		Order: -1,
		LogConfig: &structs.LogConfig{
			MaxFiles:      2,
			MaxFileSizeMB: 2,
		},
		Resources: structs.DefaultResources(),
	}

	return task
}
