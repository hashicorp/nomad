package taskenv

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// InterpolateServices returns an interpolated copy of services and checks with
// values from the task's environment.
func InterpolateServices(taskEnv *TaskEnv, services []*structs.Service) []*structs.Service {
	// Guard against not having a valid taskEnv. This can be the case if the
	// PreKilling or Exited hook is run before Poststart.
	if taskEnv == nil || len(services) == 0 {
		return nil
	}

	interpolated := make([]*structs.Service, len(services))

	for i, origService := range services {
		// Create a copy as we need to reinterpolate every time the
		// environment changes
		service := origService.Copy()

		for _, check := range service.Checks {
			check.Name = taskEnv.ReplaceEnv(check.Name)
			check.Type = taskEnv.ReplaceEnv(check.Type)
			check.Command = taskEnv.ReplaceEnv(check.Command)
			check.Args = taskEnv.ParseAndReplace(check.Args)
			check.Path = taskEnv.ReplaceEnv(check.Path)
			check.Protocol = taskEnv.ReplaceEnv(check.Protocol)
			check.PortLabel = taskEnv.ReplaceEnv(check.PortLabel)
			check.InitialStatus = taskEnv.ReplaceEnv(check.InitialStatus)
			check.Method = taskEnv.ReplaceEnv(check.Method)
			check.GRPCService = taskEnv.ReplaceEnv(check.GRPCService)
			if len(check.Header) > 0 {
				header := make(map[string][]string, len(check.Header))
				for k, vs := range check.Header {
					newVals := make([]string, len(vs))
					for i, v := range vs {
						newVals[i] = taskEnv.ReplaceEnv(v)
					}
					header[taskEnv.ReplaceEnv(k)] = newVals
				}
				check.Header = header
			}
		}

		service.Name = taskEnv.ReplaceEnv(service.Name)
		service.PortLabel = taskEnv.ReplaceEnv(service.PortLabel)
		service.Tags = taskEnv.ParseAndReplace(service.Tags)
		service.CanaryTags = taskEnv.ParseAndReplace(service.CanaryTags)

		if len(service.Meta) > 0 {
			meta := make(map[string]string, len(service.Meta))
			for k, v := range service.Meta {
				meta[k] = taskEnv.ReplaceEnv(v)
			}
			service.Meta = meta
		}

		if len(service.CanaryMeta) > 0 {
			canaryMeta := make(map[string]string, len(service.CanaryMeta))
			for k, v := range service.CanaryMeta {
				canaryMeta[k] = taskEnv.ReplaceEnv(v)
			}
			service.CanaryMeta = canaryMeta
		}

		interpolated[i] = service
	}

	return interpolated
}
