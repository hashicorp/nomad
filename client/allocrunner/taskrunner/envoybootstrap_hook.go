package taskrunner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

var _ interfaces.TaskPrestartHook = &envoyBootstrapHook{}

// envoyBootstrapHook writes the bootstrap config for the Connect Envoy proxy
// sidecar.
type envoyBootstrapHook struct {
	alloc *structs.Allocation

	// Bootstrapping Envoy requires talking directly to Consul to generate
	// the bootstrap.json config. Runtime Envoy configuration is done via
	// Consul's gRPC endpoint.
	consulHTTPAddr string

	logger log.Logger
}

func newEnvoyBootstrapHook(alloc *structs.Allocation, consulHTTPAddr string, logger log.Logger) *envoyBootstrapHook {
	h := &envoyBootstrapHook{
		alloc:          alloc,
		consulHTTPAddr: consulHTTPAddr,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (envoyBootstrapHook) Name() string {
	return "envoy_bootstrap"
}

func (h *envoyBootstrapHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	if !req.Task.Kind.IsConnectProxy() {
		// Not a Connect proxy sidecar
		resp.Done = true
		return nil
	}

	serviceName := req.Task.Kind.Value()
	if serviceName == "" {
		return fmt.Errorf("Connect proxy sidecar does not specify service name")
	}

	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)

	var service *structs.Service
	for _, s := range tg.Services {
		if s.Name == serviceName {
			service = s
			break
		}
	}

	if service == nil {
		return fmt.Errorf("Connect proxy sidecar task exists but no services configured with a sidecar")
	}

	h.logger.Debug("bootstrapping Connect proxy sidecar", "task", req.Task.Name, "service", serviceName)

	//TODO Should connect directly to Consul if the sidecar is running on
	//     the host netns.
	grpcAddr := "unix://" + allocdir.AllocGRPCSocket

	// Envoy bootstrap configuration may contain a Consul token, so write
	// it to the secrets directory like Vault tokens.
	fn := filepath.Join(req.TaskDir.SecretsDir, "envoy_bootstrap.json")

	id := agentconsul.MakeAllocServiceID(h.alloc.ID, "group-"+tg.Name, service)
	h.logger.Debug("bootstrapping envoy", "sidecar_for", service.Name, "boostrap_file", fn, "sidecar_for_id", id, "grpc_addr", grpcAddr)

	// Since Consul services are registered asynchronously with this task
	// hook running, retry a small number of times with backoff.
	for tries := 3; ; tries-- {
		cmd := exec.CommandContext(ctx, "consul", "connect", "envoy",
			"-grpc-addr", grpcAddr,
			"-http-addr", h.consulHTTPAddr,
			"-bootstrap",
			"-sidecar-for", id,
		)

		// Redirect output to secrets/envoy_bootstrap.json
		fd, err := os.Create(fn)
		if err != nil {
			return fmt.Errorf("error creating secrets/envoy_bootstrap.json for envoy: %v", err)
		}
		cmd.Stdout = fd

		buf := bytes.NewBuffer(nil)
		cmd.Stderr = buf

		// Generate bootstrap
		err = cmd.Run()

		// Close bootstrap.json
		fd.Close()

		if err == nil {
			// Happy path! Bootstrap was created, exit.
			break
		}

		// Check for error from command
		if tries == 0 {
			h.logger.Error("error creating bootstrap configuration for Connect proxy sidecar", "error", err, "stderr", buf.String())

			// Cleanup the bootstrap file. An errors here is not
			// important as (a) we test to ensure the deletion
			// occurs, and (b) the file will either be rewritten on
			// retry or eventually garbage collected if the task
			// fails.
			os.Remove(fn)

			// ExitErrors are recoverable since they indicate the
			// command was runnable but exited with a unsuccessful
			// error code.
			_, recoverable := err.(*exec.ExitError)
			return structs.NewRecoverableError(
				fmt.Errorf("error creating bootstrap configuration for Connect proxy sidecar: %v", err),
				recoverable,
			)
		}

		// Sleep before retrying to give Consul services time to register
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			// Killed before bootstrap, exit without setting Done
			return nil
		}
	}

	// Bootstrap written. Mark as done and move on.
	resp.Done = true
	return nil
}
