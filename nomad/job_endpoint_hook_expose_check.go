// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

type jobExposeCheckHook struct{}

func (jobExposeCheckHook) Name() string {
	return "expose-check"
}

// Mutate will scan every task group for group-services which have checks defined
// that have the Expose field configured, and generate expose path configurations
// extrapolated from those check definitions.
func (jobExposeCheckHook) Mutate(job *structs.Job) (_ *structs.Job, warnings []error, err error) {
	for _, tg := range job.TaskGroups {
		for _, s := range tg.Services {
			for i, c := range s.Checks {
				if c.Expose {
					// TG isn't validated yet, but validation
					// may depend on mutation results.
					// Do basic validation here and skip mutation,
					// so Validate can return a meaningful error
					// messages
					if !s.Connect.HasSidecar() {
						continue
					}

					if exposePath, err := exposePathForCheck(tg, s, c, i); err != nil {
						return nil, nil, err
					} else if exposePath != nil {
						serviceExposeConfig := serviceExposeConfig(s)
						// insert only if not already present - required for job
						// updates which would otherwise create duplicates
						if !containsExposePath(serviceExposeConfig.Paths, *exposePath) {
							serviceExposeConfig.Paths = append(
								serviceExposeConfig.Paths, *exposePath,
							)
						}
					}
				}
			}
		}
	}
	return job, nil, nil
}

// Validate will ensure:
//   - The job contains valid network configuration for each task group in which
//     an expose path is configured. The network must be of type bridge mode.
//   - The check Expose field is configured only for connect-enabled group-services.
func (jobExposeCheckHook) Validate(job *structs.Job) (warnings []error, err error) {
	for _, tg := range job.TaskGroups {
		// Make sure any group that contains a group-service that enables expose
		// is configured with one network that is in "bridge" mode. This check
		// is being done independently of the preceding Connect task injection
		// hook, because at some point in the future Connect will not require the
		// use of network namespaces, whereas the use of "expose" does not make
		// sense without the use of network namespace.
		if err := tgValidateUseOfBridgeMode(tg); err != nil {
			return nil, err
		}
		// Make sure any group-service that contains a check that enables expose
		// is connect-enabled and does not specify a custom sidecar task. We only
		// support the expose feature when using the built-in Envoy integration.
		if err := tgValidateUseOfCheckExpose(tg); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// serviceExposeConfig digs through s to extract the connect sidecar service proxy
// expose configuration. It is not required of the user to provide this, so it
// is created on demand here as needed in the case where any service check exposes
// itself.
//
// The service, connect, and sidecar_service are assumed not to be nil, as they
// are enforced in previous hooks / validation.
func serviceExposeConfig(s *structs.Service) *structs.ConsulExposeConfig {
	if s.Connect.SidecarService.Proxy == nil {
		s.Connect.SidecarService.Proxy = new(structs.ConsulProxy)
	}
	if s.Connect.SidecarService.Proxy.Expose == nil {
		s.Connect.SidecarService.Proxy.Expose = new(structs.ConsulExposeConfig)
	}
	return s.Connect.SidecarService.Proxy.Expose
}

// containsExposePath returns true if path is contained in paths.
func containsExposePath(paths []structs.ConsulExposePath, path structs.ConsulExposePath) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

// tgValidateUseOfCheckExpose ensures that any service check in tg making use
// of the expose field is within an appropriate context to do so. The check must
// be a group level check, and must use the builtin envoy proxy.
func tgValidateUseOfCheckExpose(tg *structs.TaskGroup) error {
	// validation for group services (which must use built-in connect proxy)
	for _, s := range tg.Services {
		for _, check := range s.Checks {
			if check.Expose && !s.Connect.HasSidecar() {
				return fmt.Errorf(
					"exposed service check %s->%s->%s requires use of sidecar_proxy",
					tg.Name, s.Name, check.Name,
				)
			}
		}
	}

	// validation for task services (which must not be configured to use Expose)
	for _, t := range tg.Tasks {
		for _, s := range t.Services {
			for _, check := range s.Checks {
				if check.Expose {
					return fmt.Errorf(
						"exposed service check %s[%s]->%s->%s is not a task-group service",
						tg.Name, t.Name, s.Name, check.Name,
					)
				}
			}
		}
	}
	return nil
}

// tgValidateUseOfBridgeMode ensures there is exactly 1 network configured for
// the task group, and that it makes use of "bridge" mode (i.e. enables network
// namespaces).
func tgValidateUseOfBridgeMode(tg *structs.TaskGroup) error {
	if tgUsesExposeCheck(tg) {
		if len(tg.Networks) != 1 {
			return fmt.Errorf("group %q must specify one bridge network for exposing service check(s)", tg.Name)
		}
		if tg.Networks[0].Mode != "bridge" {
			return fmt.Errorf("group %q must use bridge network for exposing service check(s)", tg.Name)
		}
	}
	return nil
}

// tgUsesExposeCheck returns true if any group service in the task group makes
// use of the expose field.
func tgUsesExposeCheck(tg *structs.TaskGroup) bool {
	for _, s := range tg.Services {
		for _, check := range s.Checks {
			if check.Expose {
				return true
			}
		}
	}
	return false
}

// checkIsExposable returns true if check is qualified for automatic generation
// of connect proxy expose path configuration based on configured consul checks.
// To qualify, the check must be of type "http" or "grpc", and must have a Path
// configured.
func checkIsExposable(check *structs.ServiceCheck) bool {
	switch strings.ToLower(check.Type) {
	case "grpc", "http":
		return strings.HasPrefix(check.Path, "/")
	default:
		return false
	}
}

// exposePathForCheck extrapolates the necessary expose path configuration for
// the given consul service check. If the check is not compatible, nil is
// returned.
func exposePathForCheck(tg *structs.TaskGroup, s *structs.Service, check *structs.ServiceCheck, i int) (*structs.ConsulExposePath, error) {
	if !checkIsExposable(check) {
		return nil, nil
	}

	// Borrow some of the validation before we start manipulating the group
	// network, which needs to exist once.
	if err := tgValidateUseOfBridgeMode(tg); err != nil {
		return nil, err
	}

	// If the check is exposable but doesn't have a port label set build
	// a port with a generated label, add it to the group's Dynamic ports
	// and set the check port label to the generated label.
	//
	// This lets PortLabel be optional for any exposed check.
	if check.PortLabel == "" {

		// Note: because the check label is not set yet, and we want to create a
		// deterministic label based on the check itself, use the index of the check
		// on the service as part of the service name as input into Hash, ensuring
		// the hash for the check is unique.
		suffix := check.Hash(fmt.Sprintf("%s_%d", s.Name, i))[:6]
		port := structs.Port{
			HostNetwork: "default",
			Label:       fmt.Sprintf("svc_%s_ck_%s", s.Name, suffix),
			To:          -1,
		}

		tg.Networks[0].DynamicPorts = append(tg.Networks[0].DynamicPorts, port)
		check.PortLabel = port.Label
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
	if mapping := tg.Networks.Port(s.PortLabel); mapping.Value <= 0 { // try looking up by port label
		if port, _ = strconv.Atoi(s.PortLabel); port <= 0 { // then try direct port value
			return nil, fmt.Errorf(
				"unable to determine local service port for service check %s->%s->%s",
				tg.Name, s.Name, check.Name,
			)
		}
	} else {
		port = mapping.Value
	}

	// The Path, Protocol, and PortLabel are just copied over from the service
	// check definition.
	return &structs.ConsulExposePath{
		Path:          check.Path,
		Protocol:      check.Protocol,
		LocalPathPort: port,
		ListenerPort:  check.PortLabel,
	}, nil
}
