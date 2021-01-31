package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/envoy"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/pkg/errors"
)

const (
	// defaultConnectTimeout is the default amount of time a connect gateway will
	// wait for a response from an upstream service (same as consul)
	defaultConnectTimeout = 5 * time.Second
)

// connectSidecarResources returns the set of resources used by default for
// the Consul Connect sidecar task
func connectSidecarResources() *structs.Resources {
	return &structs.Resources{
		CPU:      250,
		MemoryMB: 128,
	}
}

// connectSidecarDriverConfig is the driver configuration used by the injected
// connect proxy sidecar task.
func connectSidecarDriverConfig() map[string]interface{} {
	return map[string]interface{}{
		"image": envoy.SidecarConfigVar,
		"args": []interface{}{
			"-c", structs.EnvoyBootstrapPath,
			"-l", "${meta.connect.log_level}",
			"--concurrency", "${meta.connect.proxy_concurrency}",
			"--disable-hot-restart",
		},
	}
}

// connectGatewayDriverConfig is the Docker driver configuration used by the
// injected connect proxy sidecar task.
//
// A gateway may run in a group with bridge or host networking, and if host
// networking is being used the network_mode driver configuration is set here.
func connectGatewayDriverConfig(hostNetwork bool) map[string]interface{} {
	m := map[string]interface{}{
		"image": envoy.GatewayConfigVar,
		"args": []interface{}{
			"-c", structs.EnvoyBootstrapPath,
			"-l", "${meta.connect.log_level}",
			"--concurrency", "${meta.connect.proxy_concurrency}",
			"--disable-hot-restart",
		},
	}

	if hostNetwork {
		m["network_mode"] = "host"
	}

	return m
}

// connectSidecarVersionConstraint is used when building the sidecar task to ensure
// the proper Consul version is used that supports the necessary Connect
// features. This includes bootstrapping envoy with a unix socket for Consul's
// gRPC xDS API.
func connectSidecarVersionConstraint() *structs.Constraint {
	return &structs.Constraint{
		LTarget: "${attr.consul.version}",
		RTarget: ">= 1.6.0-beta1",
		Operand: structs.ConstraintSemver,
	}
}

// connectGatewayVersionConstraint is used when building a connect gateway
// task to ensure proper Consul version is used that supports Connect Gateway
// features. This includes making use of Consul Configuration Entries of type
// {ingress,terminating,mesh}-gateway.
func connectGatewayVersionConstraint() *structs.Constraint {
	return &structs.Constraint{
		LTarget: "${attr.consul.version}",
		RTarget: ">= 1.8.0",
		Operand: structs.ConstraintSemver,
	}
}

// jobConnectHook implements a job Mutating and Validating admission controller
type jobConnectHook struct{}

func (jobConnectHook) Name() string {
	return "connect"
}

func (jobConnectHook) Mutate(job *structs.Job) (*structs.Job, []error, error) {
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

func (jobConnectHook) Validate(job *structs.Job) ([]error, error) {
	var warnings []error

	for _, g := range job.TaskGroups {
		if w, err := groupConnectValidate(g); err != nil {
			return nil, err
		} else if w != nil {
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
	return t.Kind == structs.NewTaskKind(structs.ConnectProxyPrefix, svc)
}

func hasGatewayTaskForService(tg *structs.TaskGroup, svc string) bool {
	for _, t := range tg.Tasks {
		switch {
		case isIngressGatewayForService(t, svc):
			return true
		case isTerminatingGatewayForService(t, svc):
			return true
		}
		// mesh later
	}
	return false
}

func isIngressGatewayForService(t *structs.Task, svc string) bool {
	return t.Kind == structs.NewTaskKind(structs.ConnectIngressPrefix, svc)
}

func isTerminatingGatewayForService(t *structs.Task, svc string) bool {
	return t.Kind == structs.NewTaskKind(structs.ConnectTerminatingPrefix, svc)
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

func injectPort(group *structs.TaskGroup, label string) {
	// check that port hasn't already been defined before adding it to tg
	for _, p := range group.Networks[0].DynamicPorts {
		if p.Label == label {
			return
		}
	}

	// inject a port of label that maps inside the bridge namespace
	group.Networks[0].DynamicPorts = append(group.Networks[0].DynamicPorts, structs.Port{
		Label: label,
		// -1 is a sentinel value to instruct the
		// scheduler to map the host's dynamic port to
		// the same port in the netns.
		To: -1,
	})
}

// probably need to hack this up to look for checks on the service, and if they
// qualify, configure a port for envoy to use to expose their paths.
func groupConnectHook(job *structs.Job, g *structs.TaskGroup) error {
	// Create an environment interpolator with what we have at submission time.
	// This should only be used to interpolate connect service names which are
	// used in sidecar or gateway task names. Note that the service name might
	// also be interpolated with job specifics during service canonicalization.
	env := taskenv.NewEmptyBuilder().UpdateTask(&structs.Allocation{
		Job:       job,
		TaskGroup: g.Name,
	}, nil).Build()

	for _, service := range g.Services {
		switch {
		// mutate depending on what the connect block is being used for

		case service.Connect.HasSidecar():
			// interpolate the connect service name, which is used to create
			// a name of an injected sidecar task
			service.Name = env.ReplaceEnv(service.Name)

			// Check to see if the sidecar task already exists
			task := getSidecarTaskForService(g, service.Name)

			// If the task doesn't already exist, create a new one and add it to the job
			if task == nil {
				task = newConnectSidecarTask(service.Name)

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

			// create a port for the sidecar task's proxy port
			injectPort(g, fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, service.Name))

		case service.Connect.IsNative():
			// find the task backing this connect native service and set the kind
			nativeTaskName := service.TaskName
			if t, err := getNamedTaskForNativeService(g, service.Name, nativeTaskName); err != nil {
				return err
			} else {
				t.Kind = structs.NewTaskKind(structs.ConnectNativePrefix, service.Name)
				service.TaskName = t.Name // in case the task was inferred
			}

		case service.Connect.IsGateway():
			// interpolate the connect service name, which is used to create
			// a name of an injected gateway task
			service.Name = env.ReplaceEnv(service.Name)

			// detect whether the group is in host networking mode, which will
			// require tweaking the default gateway task config
			netHost := g.Networks[0].Mode == "host"

			if !netHost && service.Connect.IsGateway() {
				// Modify the gateway proxy service configuration to automatically
				// do the correct envoy bind address plumbing when inside a net
				// namespace, but only if things are not explicitly configured.
				service.Connect.Gateway.Proxy = gatewayProxyForBridge(service.Connect.Gateway)
			}

			// Inject a port whether bridge or host network (if not already set).
			// This port is accessed by the magic of Connect plumbing so it seems
			// reasonable to keep the magic alive here.
			if service.Connect.IsTerminating() && service.PortLabel == "" {
				// Inject a dynamic port for the terminating gateway.
				portLabel := fmt.Sprintf("%s-%s", structs.ConnectTerminatingPrefix, service.Name)
				service.PortLabel = portLabel
				injectPort(g, portLabel)
			}

			// inject the gateway task only if it does not yet already exist
			if !hasGatewayTaskForService(g, service.Name) {
				prefix := service.Connect.Gateway.Prefix()
				task := newConnectGatewayTask(prefix, service.Name, netHost)
				g.Tasks = append(g.Tasks, task)

				// the connect.sidecar_task stanza can also be used to configure
				// a custom task to use as a gateway proxy
				if service.Connect.SidecarTask != nil {
					service.Connect.SidecarTask.MergeIntoTask(task)
				}

				task.Canonicalize(job, g)
			}
		}
	}

	// re-canonicalize group network since this hook runs after canonicalizaton
	g.Networks[0].Canonicalize()
	return nil
}

// gatewayProxyIsDefault returns false if any of these gateway proxy configuration
// have been modified from their default values, indicating the operator wants
// custom behavior. Otherwise, we assume the operator wants Nomad to do the Right
// Thing, setting the configuration automatically.
//
// - envoy_gateway_no_default_bind
// - envoy_gateway_bind_tagged_addresses
// - envoy_gateway_bind_addresses
func gatewayProxyIsDefault(proxy *structs.ConsulGatewayProxy) bool {
	if proxy == nil {
		return true
	}
	if !proxy.EnvoyGatewayNoDefaultBind &&
		!proxy.EnvoyGatewayBindTaggedAddresses &&
		len(proxy.EnvoyGatewayBindAddresses) == 0 {
		return true
	}
	return false
}

// gatewayProxyForBridge scans an existing gateway proxy configuration and tweaks
// it given an associated configuration entry so that it works as intended from
// inside a network namespace.
func gatewayProxyForBridge(gateway *structs.ConsulGateway) *structs.ConsulGatewayProxy {
	if gateway == nil {
		return nil
	}

	// operator has supplied custom proxy configuration, just use that without
	// modification
	if !gatewayProxyIsDefault(gateway.Proxy) {
		return gateway.Proxy
	}

	// copy over unrelated fields if proxy block exists
	proxy := new(structs.ConsulGatewayProxy)
	if gateway.Proxy != nil {
		proxy.ConnectTimeout = gateway.Proxy.ConnectTimeout
		proxy.EnvoyDNSDiscoveryType = gateway.Proxy.EnvoyDNSDiscoveryType
		proxy.Config = gateway.Proxy.Config
	}

	// set default connect timeout if not set
	if proxy.ConnectTimeout == nil {
		proxy.ConnectTimeout = helper.TimeToPtr(defaultConnectTimeout)
	}

	// magically configure bind address(es) for bridge networking, per gateway type
	// non-default configuration is gated above
	switch {
	case gateway.Ingress != nil:
		proxy.EnvoyGatewayNoDefaultBind = true
		proxy.EnvoyGatewayBindTaggedAddresses = false
		proxy.EnvoyGatewayBindAddresses = gatewayBindAddressesIngress(gateway.Ingress)
	case gateway.Terminating != nil:
		proxy.EnvoyGatewayNoDefaultBind = true
		proxy.EnvoyGatewayBindTaggedAddresses = false
		proxy.EnvoyGatewayBindAddresses = map[string]*structs.ConsulGatewayBindAddress{
			"default": {
				Address: "0.0.0.0",
				Port:    -1, // filled in later with dynamic port
			}}
	}
	// later: mesh

	return proxy
}

func gatewayBindAddressesIngress(ingress *structs.ConsulIngressConfigEntry) map[string]*structs.ConsulGatewayBindAddress {
	if ingress == nil || len(ingress.Listeners) == 0 {
		return make(map[string]*structs.ConsulGatewayBindAddress)
	}

	addresses := make(map[string]*structs.ConsulGatewayBindAddress)
	for _, listener := range ingress.Listeners {
		port := listener.Port
		for _, service := range listener.Services {
			addresses[service.Name] = &structs.ConsulGatewayBindAddress{
				Address: "0.0.0.0",
				Port:    port,
			}
		}
	}
	return addresses
}

func newConnectGatewayTask(prefix, service string, netHost bool) *structs.Task {
	return &structs.Task{
		// Name is used in container name so must start with '[A-Za-z0-9]'
		Name:          fmt.Sprintf("%s-%s", prefix, service),
		Kind:          structs.NewTaskKind(prefix, service),
		Driver:        "docker",
		Config:        connectGatewayDriverConfig(netHost),
		ShutdownDelay: 5 * time.Second,
		LogConfig: &structs.LogConfig{
			MaxFiles:      2,
			MaxFileSizeMB: 2,
		},
		Resources: connectSidecarResources(),
		Constraints: structs.Constraints{
			connectGatewayVersionConstraint(),
		},
	}
}

func newConnectSidecarTask(service string) *structs.Task {
	return &structs.Task{
		// Name is used in container name so must start with '[A-Za-z0-9]'
		Name:          fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, service),
		Kind:          structs.NewTaskKind(structs.ConnectProxyPrefix, service),
		Driver:        "docker",
		Config:        connectSidecarDriverConfig(),
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
			connectSidecarVersionConstraint(),
		},
	}
}

func groupConnectValidate(g *structs.TaskGroup) (warnings []error, err error) {
	for _, s := range g.Services {
		switch {
		case s.Connect.HasSidecar():
			if err := groupConnectSidecarValidate(g); err != nil {
				return nil, err
			}
		case s.Connect.IsNative():
			if err := groupConnectNativeValidate(g, s); err != nil {
				return nil, err
			}
		case s.Connect.IsGateway():
			if err := groupConnectGatewayValidate(g); err != nil {
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

func groupConnectGatewayValidate(g *structs.TaskGroup) error {
	// the group needs to be either bridge or host mode so we know how to configure
	// the docker driver config

	if n := len(g.Networks); n != 1 {
		return fmt.Errorf("Consul Connect gateways require exactly 1 network, found %d in group %q", n, g.Name)
	}

	modes := []string{"bridge", "host"}
	if !helper.SliceStringContains(modes, g.Networks[0].Mode) {
		return fmt.Errorf(`Consul Connect Gateway service requires Task Group with network mode of type "bridge" or "host"`)
	}

	return nil
}
