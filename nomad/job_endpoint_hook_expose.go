package nomad

import (
	"strconv"

	"github.com/pkg/errors"

	"github.com/hashicorp/nomad/nomad/structs"
)

// jobExposeHook implements a job Mutating and Validating admission controller.
type jobExposeHook struct{}

func (jobExposeHook) Name() string {
	return "expose"
}

// Mutate will extrapolate and append each of the the necessary expose.path
// configurations for each compatible service check in each service in each
// task group.
func (jobExposeHook) Mutate(job *structs.Job) (*structs.Job, []error, error) {
	for _, tg := range job.TaskGroups {
		for _, s := range tg.Services {
			if serviceEnablesExposeChecks(s) {
				for _, check := range s.Checks {
					// create an expose path for each check that is compatible
					if ePath, err := exposePathForCheck(tg, s, check); err != nil {
						return nil, nil, err
					} else if ePath != nil {
						// insert only if compatible & not already present
						if !containsExposePath(s.Connect.SidecarService.Proxy.Expose.Paths, *ePath) {
							s.Connect.SidecarService.Proxy.Expose.Paths = append(
								s.Connect.SidecarService.Proxy.Expose.Paths,
								*ePath,
							)
						}
					}
				}
			}
		}
	}
	return job, nil, nil
}

// Validate will ensure the job contains valid network configuration for each
// task group in which an expose path is configured. The network must be of type
// bridge mode.
func (jobExposeHook) Validate(job *structs.Job) ([]error, error) {
	// Make sure expose config exists only along with a bridge network. We could
	// also validate existence of named port mappings, but Mutate will also
	// necessarily make that check when it looks up the service port.
	for _, tg := range job.TaskGroups {
		if tgEnablesExpose(tg) {
			if mode, group, ok := tgUsesBridgeNetwork(tg); !ok {
				return nil, errors.Errorf(
					"expose configuration requires bridge network, found %q network in task group %q",
					mode, group,
				)
			}
		}
	}
	return nil, nil
}

func containsExposePath(paths []structs.ConsulExposePath, path structs.ConsulExposePath) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

// exposePathForCheck extrapolates the necessary expose path configuration for
// the given consul service check. If the check is not compatible, nil is
// returned.
func exposePathForCheck(tg *structs.TaskGroup, s *structs.Service, check *structs.ServiceCheck) (*structs.ConsulExposePath, error) {
	if !checkIsExposable(check) {
		return nil, nil
	}

	// Determine the local service port (i.e. what port the service is actually
	// listening to inside the network namespace).
	//
	// Similar logic exists in getAddress of client.go which is used for
	// creating check & service registration objects.
	//
	// The difference here is the address is predestined to be localhost since
	// it is binding inside the namespace.
	var port int
	if _, port = tg.Networks.Port(s.PortLabel); port <= 0 { // try looking up by port label
		if port, _ = strconv.Atoi(s.PortLabel); port <= 0 { // then try direct port value
			return nil, errors.Errorf(
				"unable to determine local service port for check %q of service %q in group %q",
				check.Name, s.Name, tg.Name,
			)
		}
	}

	// The Path, Protocol, and PortLabel are just copied over from the service
	// check definition. It is required that the user configure their own port
	// mapping for each check, including setting the 'to = -1' sentinel value
	// enabling the network namespace pass-through.
	return &structs.ConsulExposePath{
		Path:          check.Path,
		Protocol:      check.Protocol,
		LocalPathPort: port,
		ListenerPort:  check.PortLabel,
	}, nil
}

// checkIsExposable returns true if check is qualified for automatic generation
// of connect proxy expose path configuration based on configured consul checks.
// To qualify, the check must be of type "http" or "grpc", and must have a Path
// configured.
func checkIsExposable(check *structs.ServiceCheck) bool {
	switch check.Type {
	case "grpc", "http":
		return check.Path != ""
	default:
		return false
	}
}

// tgEnablesExpose returns true if any group-level service in tg makes use of
// the connect proxy expose configuration.
func tgEnablesExpose(tg *structs.TaskGroup) bool {
	for _, s := range tg.Services {
		if serviceEnablesExpose(s) {
			return true
		}
	}
	return false
}

// serviceEnablesExpose returns true if s is configured to expose endpoints
// through the connect proxy, whether with explicit path configurations or using
// automatic configuration based on consul checks.
func serviceEnablesExpose(s *structs.Service) bool {
	exposeConfig := serviceExposeConfig(s)
	if exposeConfig == nil {
		return false
	}
	return exposeConfig.Checks || len(exposeConfig.Paths) > 0
}

// serviceEnablesExposeChecks returns true if s is configured to automatically
// expose consul service check endpoints through the connect proxy.
func serviceEnablesExposeChecks(s *structs.Service) bool {
	exposeConfig := serviceExposeConfig(s)
	if exposeConfig == nil {
		return false
	}
	return exposeConfig.Checks
}

// serviceExposeConfig digs through s to extract the connect sidecar service
// proxy expose configuration. If any layer is nil, nil is returned.
func serviceExposeConfig(s *structs.Service) *structs.ConsulExposeConfig {
	if s == nil {
		return nil
	}

	if s.Connect == nil {
		return nil
	}

	if s.Connect.SidecarService == nil {
		return nil
	}

	if s.Connect.SidecarService.Proxy == nil {
		return nil
	}

	return s.Connect.SidecarService.Proxy.Expose
}

// tgUsesBridgeNetwork detects whether the 0th network in tg uses the bridge
// networking mode, and returns the mode and name as well to the caller for
// convenient error reporting.
func tgUsesBridgeNetwork(tg *structs.TaskGroup) (string, string, bool) {
	mode := tg.Networks[0].Mode
	return mode, tg.Name, mode == "bridge"
}
