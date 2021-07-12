package taskrunner

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	ifs "github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/exptime"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/pkg/errors"
)

const envoyBootstrapHookName = "envoy_bootstrap"

const (
	// envoyBootstrapWaitTime is the amount of time this hook should wait on Consul
	// objects to be created before giving up.
	envoyBootstrapWaitTime = 60 * time.Second

	// envoyBootstrapInitialGap is the initial amount of time the envoy bootstrap
	// retry loop will wait, exponentially increasing each iteration, not including
	// jitter.
	envoyBoostrapInitialGap = 1 * time.Second

	// envoyBootstrapMaxJitter is the maximum amount of jitter applied to the
	// wait gap each iteration of the envoy bootstrap retry loop.
	envoyBootstrapMaxJitter = 500 * time.Millisecond
)

type consulTransportConfig struct {
	HTTPAddr  string // required
	Auth      string // optional, env CONSUL_HTTP_AUTH
	SSL       string // optional, env CONSUL_HTTP_SSL
	VerifySSL string // optional, env CONSUL_HTTP_SSL_VERIFY
	CAFile    string // optional, arg -ca-file
	CertFile  string // optional, arg -client-cert
	KeyFile   string // optional, arg -client-key
	Namespace string // optional, only consul Enterprise, env CONSUL_NAMESPACE
	// CAPath (dir) not supported by Nomad's config object
}

func newConsulTransportConfig(consul *config.ConsulConfig) consulTransportConfig {
	return consulTransportConfig{
		HTTPAddr:  consul.Addr,
		Auth:      consul.Auth,
		SSL:       decodeTriState(consul.EnableSSL),
		VerifySSL: decodeTriState(consul.VerifySSL),
		CAFile:    consul.CAFile,
		CertFile:  consul.CertFile,
		KeyFile:   consul.KeyFile,
		Namespace: consul.Namespace,
	}
}

type envoyBootstrapHookConfig struct {
	alloc           *structs.Allocation
	consul          consulTransportConfig
	consulNamespace string
	logger          hclog.Logger
}

func decodeTriState(b *bool) string {
	switch {
	case b == nil:
		return ""
	case *b:
		return "true"
	default:
		return "false"
	}
}

func newEnvoyBootstrapHookConfig(alloc *structs.Allocation, consul *config.ConsulConfig, consulNamespace string, logger hclog.Logger) *envoyBootstrapHookConfig {
	return &envoyBootstrapHookConfig{
		alloc:           alloc,
		consul:          newConsulTransportConfig(consul),
		consulNamespace: consulNamespace,
		logger:          logger,
	}
}

const (
	envoyBaseAdminPort      = 19000 // Consul default (bridge only)
	envoyBaseReadyPort      = 19100 // Consul default (bridge only)
	envoyAdminBindEnvPrefix = "NOMAD_ENVOY_ADMIN_ADDR_"
	envoyReadyBindEnvPrefix = "NOMAD_ENVOY_READY_ADDR_"
)

const (
	grpcConsulVariable = "CONSUL_GRPC_ADDR"
	grpcDefaultAddress = "127.0.0.1:8502"
)

// envoyBootstrapHook writes the bootstrap config for the Connect Envoy proxy
// sidecar.
type envoyBootstrapHook struct {
	// alloc is the allocation with the envoy task being bootstrapped.
	alloc *structs.Allocation

	// Bootstrapping Envoy requires talking directly to Consul to generate
	// the bootstrap.json config. Runtime Envoy configuration is done via
	// Consul's gRPC endpoint. There are many security parameters to configure
	// before contacting Consul.
	consulConfig consulTransportConfig

	// consulNamespace is the Consul namespace as set by in the job
	consulNamespace string

	// envoyBootstrapWaitTime is the total amount of time hook will wait for Consul
	envoyBootstrapWaitTime time.Duration

	// envoyBootstrapInitialGap is the initial wait gap when retyring
	envoyBoostrapInitialGap time.Duration

	// envoyBootstrapMaxJitter is the maximum amount of jitter applied to retries
	envoyBootstrapMaxJitter time.Duration

	// envoyBootstrapExpSleep controls exponential waiting
	envoyBootstrapExpSleep func(time.Duration)

	// logger is used to log things
	logger hclog.Logger
}

func newEnvoyBootstrapHook(c *envoyBootstrapHookConfig) *envoyBootstrapHook {
	return &envoyBootstrapHook{
		alloc:                   c.alloc,
		consulConfig:            c.consul,
		consulNamespace:         c.consulNamespace,
		envoyBootstrapWaitTime:  envoyBootstrapWaitTime,
		envoyBoostrapInitialGap: envoyBoostrapInitialGap,
		envoyBootstrapMaxJitter: envoyBootstrapMaxJitter,
		envoyBootstrapExpSleep:  time.Sleep,
		logger:                  c.logger.Named(envoyBootstrapHookName),
	}
}

// getConsulNamespace will resolve the Consul namespace, choosing between
//  - agent config (low precedence)
//  - task group config (high precedence)
func (h *envoyBootstrapHook) getConsulNamespace() string {
	var namespace string
	if h.consulConfig.Namespace != "" {
		namespace = h.consulConfig.Namespace
	}
	if h.consulNamespace != "" {
		namespace = h.consulNamespace
	}
	return namespace
}

func (envoyBootstrapHook) Name() string {
	return envoyBootstrapHookName
}

func isConnectKind(kind string) bool {
	switch kind {
	case structs.ConnectProxyPrefix:
		return true
	case structs.ConnectIngressPrefix:
		return true
	case structs.ConnectTerminatingPrefix:
		return true
	case structs.ConnectMeshPrefix:
		return true
	default:
		return false
	}
}

func (_ *envoyBootstrapHook) extractNameAndKind(kind structs.TaskKind) (string, string, error) {
	serviceName := kind.Value()
	serviceKind := kind.Name()

	if !isConnectKind(serviceKind) {
		return "", "", errors.New("envoy must be used as connect sidecar or gateway")
	}

	if serviceName == "" {
		return "", "", errors.New("envoy must be configured with a service name")
	}

	return serviceKind, serviceName, nil
}

func (h *envoyBootstrapHook) lookupService(svcKind, svcName string, taskEnv *taskenv.TaskEnv) (*structs.Service, error) {
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	interpolatedServices := taskenv.InterpolateServices(taskEnv, tg.Services)

	var service *structs.Service
	for _, s := range interpolatedServices {
		if s.Name == svcName {
			service = s
			break
		}
	}

	if service == nil {
		if svcKind == structs.ConnectProxyPrefix {
			return nil, errors.New("connect proxy sidecar task exists but no services configured with a sidecar")
		} else {
			return nil, errors.New("connect gateway task exists but no service associated")
		}
	}

	return service, nil
}

// Prestart creates an envoy bootstrap config file.
//
// Must be aware of both launching envoy as a sidecar proxy, as well as a connect gateway.
func (h *envoyBootstrapHook) Prestart(ctx context.Context, req *ifs.TaskPrestartRequest, resp *ifs.TaskPrestartResponse) error {
	if !req.Task.Kind.IsConnectProxy() && !req.Task.Kind.IsAnyConnectGateway() {
		// Not a Connect proxy sidecar
		resp.Done = true
		return nil
	}

	serviceKind, serviceName, err := h.extractNameAndKind(req.Task.Kind)
	if err != nil {
		return err
	}

	service, err := h.lookupService(serviceKind, serviceName, req.TaskEnv)
	if err != nil {
		return err
	}

	grpcAddr := h.grpcAddress(req.TaskEnv.EnvMap)

	h.logger.Debug("bootstrapping Consul "+serviceKind, "task", req.Task.Name, "service", serviceName)

	// Envoy runs an administrative listener. There is no way to turn this feature off.
	// https://github.com/envoyproxy/envoy/issues/1297
	envoyAdminBind := buildEnvoyAdminBind(h.alloc, serviceName, req.Task.Name, req.TaskEnv)

	// Consul configures a ready listener. There is no way to turn this feature off.
	envoyReadyBind := buildEnvoyReadyBind(h.alloc, serviceName, req.Task.Name, req.TaskEnv)

	// Set runtime environment variables for the envoy admin and ready listeners.
	resp.Env = map[string]string{
		helper.CleanEnvVar(envoyAdminBindEnvPrefix+serviceName, '_'): envoyAdminBind,
		helper.CleanEnvVar(envoyReadyBindEnvPrefix+serviceName, '_'): envoyReadyBind,
	}

	// Envoy bootstrap configuration may contain a Consul token, so write
	// it to the secrets directory like Vault tokens.
	bootstrapFilePath := filepath.Join(req.TaskDir.SecretsDir, "envoy_bootstrap.json")

	siToken, err := h.maybeLoadSIToken(req.Task.Name, req.TaskDir.SecretsDir)
	if err != nil {
		h.logger.Error("failed to generate envoy bootstrap config", "sidecar_for", service.Name)
		return errors.Wrap(err, "failed to generate envoy bootstrap config")
	}
	h.logger.Debug("check for SI token for task", "task", req.Task.Name, "exists", siToken != "")

	bootstrap := h.newEnvoyBootstrapArgs(h.alloc.TaskGroup, service, grpcAddr, envoyAdminBind, envoyReadyBind, siToken, bootstrapFilePath)
	bootstrapArgs := bootstrap.args()
	bootstrapEnv := bootstrap.env(os.Environ())

	// keep track of latest error returned from exec-ing consul envoy bootstrap
	var cmdErr error

	// Since Consul services are registered asynchronously with this task
	// hook running, retry until timeout or success.
	if backoffErr := exptime.Backoff(func() (bool, error) {

		// If hook is killed, just stop.
		select {
		case <-ctx.Done():
			return false, nil
		default:
		}

		// Prepare bootstrap command to run.
		cmd := exec.CommandContext(ctx, "consul", bootstrapArgs...)
		cmd.Env = bootstrapEnv

		// Redirect stdout to secrets/envoy_bootstrap.json.
		fd, fileErr := os.Create(bootstrapFilePath)
		if fileErr != nil {
			return false, fmt.Errorf("failed to create secrets/envoy_bootstrap.json for envoy: %w", fileErr)
		}
		cmd.Stdout = fd

		// Redirect stderr into a buffer for later reading.
		buf := bytes.NewBuffer(nil)
		cmd.Stderr = buf

		// Generate bootstrap
		cmdErr = cmd.Run()

		// Close bootstrap.json regardless of any command errors.
		_ = fd.Close()

		// Command succeeded, exit.
		if cmdErr == nil {
			// Bootstrap written. Mark as done and move on.
			resp.Done = true
			return false, nil
		}

		// Command failed, prepare for retry
		//
		// Cleanup the bootstrap file. An errors here is not
		// important as (a) we test to ensure the deletion
		// occurs, and (b) the file will either be rewritten on
		// retry or eventually garbage collected if the task
		// fails.
		_ = os.Remove(bootstrapFilePath)

		return true, cmdErr
	}, exptime.BackoffOptions{
		MaxSleepTime:   h.envoyBootstrapWaitTime,
		InitialGapSize: h.envoyBoostrapInitialGap,
		MaxJitterSize:  h.envoyBootstrapMaxJitter,
	}); backoffErr != nil {
		// Wrap the last error from Consul and set that as our status.
		_, recoverable := cmdErr.(*exec.ExitError)
		return structs.NewRecoverableError(
			fmt.Errorf("error creating bootstrap configuration for Connect proxy sidecar: %v", cmdErr),
			recoverable,
		)
	}

	return nil
}

// buildEnvoyAdminBind determines a unique port for use by the envoy admin listener.
//
// This listener will be bound to 127.0.0.2.
func buildEnvoyAdminBind(alloc *structs.Allocation, service, task string, env *taskenv.TaskEnv) string {
	return buildEnvoyBind(alloc, "127.0.0.2", service, task, env, envoyBaseAdminPort)
}

// buildEnvoyAdminBind determines a unique port for use by the envoy ready listener.
//
// This listener will be bound to 127.0.0.1.
func buildEnvoyReadyBind(alloc *structs.Allocation, service, task string, env *taskenv.TaskEnv) string {
	return buildEnvoyBind(alloc, "127.0.0.1", service, task, env, envoyBaseReadyPort)
}

// buildEnvoyBind is used to determine a unique port for an envoy listener.
//
// In bridge mode, if multiple sidecars are running, the bind addresses need
// to be unique within the namespace, so we simply start at basePort and increment
// by the index of the task.
//
// In host mode, use the port provided through the service definition, which can
// be a port chosen by Nomad.
func buildEnvoyBind(alloc *structs.Allocation, ifce, service, task string, taskEnv *taskenv.TaskEnv, basePort int) string {
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	port := basePort
	switch tg.Networks[0].Mode {
	case "host":
		interpolatedServices := taskenv.InterpolateServices(taskEnv, tg.Services)
		for _, svc := range interpolatedServices {
			if svc.Name == service {
				mapping := tg.Networks.Port(svc.PortLabel)
				port = mapping.Value
				break
			}
		}
	default:
		for idx, tgTask := range tg.Tasks {
			if tgTask.Name == task {
				port += idx
				break
			}
		}
	}
	return net.JoinHostPort(ifce, strconv.Itoa(port))
}

func (h *envoyBootstrapHook) writeConfig(filename, config string) error {
	if err := ioutil.WriteFile(filename, []byte(config), 0440); err != nil {
		_ = os.Remove(filename)
		return err
	}
	return nil
}

func (h *envoyBootstrapHook) execute(cmd *exec.Cmd) (string, error) {
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		_, recoverable := err.(*exec.ExitError)
		// ExitErrors are recoverable since they indicate the
		// command was runnable but exited with a unsuccessful
		// error code.
		return stderr.String(), structs.NewRecoverableError(err, recoverable)
	}
	return stdout.String(), nil
}

// grpcAddress determines the Consul gRPC endpoint address to use.
//
// In host networking this will default to 127.0.0.1:8502.
// In bridge/cni networking this will default to unix://<socket>.
// In either case, CONSUL_GRPC_ADDR will override the default.
func (h *envoyBootstrapHook) grpcAddress(env map[string]string) string {
	if address := env[grpcConsulVariable]; address != "" {
		return address
	}

	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	switch tg.Networks[0].Mode {
	case "host":
		return grpcDefaultAddress
	default:
		return "unix://" + allocdir.AllocGRPCSocket
	}
}

func (h *envoyBootstrapHook) proxyServiceID(group string, service *structs.Service) string {
	return agentconsul.MakeAllocServiceID(h.alloc.ID, "group-"+group, service)
}

// newEnvoyBootstrapArgs is used to prepare for the invocation of the
// 'consul connect envoy' command with arguments which will bootstrap the connect
// proxy or gateway.
//
// https://www.consul.io/commands/connect/envoy#consul-connect-envoy
func (h *envoyBootstrapHook) newEnvoyBootstrapArgs(
	group string, service *structs.Service,
	grpcAddr, envoyAdminBind, envoyReadyBind, siToken, filepath string,
) envoyBootstrapArgs {
	var (
		sidecarForID string // sidecar only
		gateway      string // gateway only
		proxyID      string // gateway only
		namespace    string
	)

	namespace = h.getConsulNamespace()
	id := h.proxyServiceID(group, service)

	switch {
	case service.Connect.HasSidecar():
		sidecarForID = id
	case service.Connect.IsIngress():
		proxyID = id
		gateway = "ingress"
	case service.Connect.IsTerminating():
		proxyID = id
		gateway = "terminating"
	case service.Connect.IsMesh():
		proxyID = id
		gateway = "mesh"
	}

	h.logger.Info("bootstrapping envoy",
		"sidecar_for", service.Name, "bootstrap_file", filepath,
		"sidecar_for_id", sidecarForID, "grpc_addr", grpcAddr,
		"admin_bind", envoyAdminBind, "ready_bind", envoyReadyBind,
		"gateway", gateway, "proxy_id", proxyID, "namespace", namespace,
	)

	return envoyBootstrapArgs{
		consulConfig:   h.consulConfig,
		sidecarFor:     sidecarForID,
		grpcAddr:       grpcAddr,
		envoyAdminBind: envoyAdminBind,
		envoyReadyBind: envoyReadyBind,
		siToken:        siToken,
		gateway:        gateway,
		proxyID:        proxyID,
		namespace:      namespace,
	}
}

// envoyBootstrapArgs is used to accumulate CLI arguments that will be passed
// along to the exec invocation of consul which will then generate the bootstrap
// configuration file for envoy.
type envoyBootstrapArgs struct {
	consulConfig   consulTransportConfig
	sidecarFor     string // sidecars only
	grpcAddr       string
	envoyAdminBind string
	envoyReadyBind string
	siToken        string
	gateway        string // gateways only
	proxyID        string // gateways only
	namespace      string
}

// args returns the CLI arguments consul needs in the correct order, with the
// -token argument present or not present depending on whether it is set.
func (e envoyBootstrapArgs) args() []string {
	arguments := []string{
		"connect",
		"envoy",
		"-grpc-addr", e.grpcAddr,
		"-http-addr", e.consulConfig.HTTPAddr,
		"-admin-bind", e.envoyAdminBind,
		"-address", e.envoyReadyBind,
		"-bootstrap",
	}

	if v := e.sidecarFor; v != "" {
		arguments = append(arguments, "-sidecar-for", v)
	}

	if v := e.gateway; v != "" {
		arguments = append(arguments, "-gateway", v)
	}

	if v := e.proxyID; v != "" {
		arguments = append(arguments, "-proxy-id", v)
	}

	if v := e.siToken; v != "" {
		arguments = append(arguments, "-token", v)
	}

	if v := e.consulConfig.CAFile; v != "" {
		arguments = append(arguments, "-ca-file", v)
	}

	if v := e.consulConfig.CertFile; v != "" {
		arguments = append(arguments, "-client-cert", v)
	}

	if v := e.consulConfig.KeyFile; v != "" {
		arguments = append(arguments, "-client-key", v)
	}

	if v := e.namespace; v != "" {
		arguments = append(arguments, "-namespace", v)
	}

	return arguments
}

// env creates the context of environment variables to be used when exec-ing
// the consul command for generating the envoy bootstrap config. It is expected
// the value of os.Environ() is passed in to be appended to. Because these are
// appended at the end of what will be passed into Cmd.Env, they will override
// any pre-existing values (i.e. what the Nomad agent was launched with).
// https://golang.org/pkg/os/exec/#Cmd
func (e envoyBootstrapArgs) env(env []string) []string {
	if v := e.consulConfig.Auth; v != "" {
		env = append(env, fmt.Sprintf("%s=%s", "CONSUL_HTTP_AUTH", v))
	}
	if v := e.consulConfig.SSL; v != "" {
		env = append(env, fmt.Sprintf("%s=%s", "CONSUL_HTTP_SSL", v))
	}
	if v := e.consulConfig.VerifySSL; v != "" {
		env = append(env, fmt.Sprintf("%s=%s", "CONSUL_HTTP_SSL_VERIFY", v))
	}
	if v := e.namespace; v != "" {
		env = append(env, fmt.Sprintf("%s=%s", "CONSUL_NAMESPACE", v))
	}
	return env
}

// maybeLoadSIToken reads the SI token saved to disk in the secrets directory
// by the service identities prestart hook. This envoy bootstrap hook blocks
// until the sids hook completes, so if the SI token is required to exist (i.e.
// Consul ACLs are enabled), it will be in place by the time we try to read it.
func (h *envoyBootstrapHook) maybeLoadSIToken(task, dir string) (string, error) {
	tokenPath := filepath.Join(dir, sidsTokenFile)
	token, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		if !os.IsNotExist(err) {
			h.logger.Error("failed to load SI token", "task", task, "error", err)
			return "", errors.Wrapf(err, "failed to load SI token for %s", task)
		}
		h.logger.Trace("no SI token to load", "task", task)
		return "", nil // token file does not exist
	}
	h.logger.Trace("recovered pre-existing SI token", "task", task)
	return string(token), nil
}
