package taskrunner

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
)

var _ interfaces.TaskPrestartHook = &connectHook{}

// connectHook writes the bootstrap config for the envoy sidecar proxy
type connectHook struct {
	alloc          *structs.Allocation
	consulHTTPAddr string

	logger log.Logger
}

func newConnectHook(logger log.Logger, alloc *structs.Allocation, consulHTTPAddr string) *connectHook {
	h := &connectHook{
		alloc:          alloc,
		consulHTTPAddr: consulHTTPAddr,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (connectHook) Name() string {
	return "connect"
}

func (h *connectHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	//TODO(schmichael) this is a pretty silly way of only running for one task
	if req.Task.Name != "nomad_envoy" {
		resp.Done = true
		return nil
	}

	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)

	var service *structs.Service
	for _, s := range tg.Services {
		if s.Connect.HasSidecar() {
			service = s
			break
		}
	}

	if service == nil {
		return fmt.Errorf("envoy sidecar task exists but no services configured with a sidecar")
	}

	canary := false
	if h.alloc.DeploymentStatus != nil {
		canary = h.alloc.DeploymentStatus.Canary
	}

	//TODO(schmichael) Will this path work for all drivers?
	grpcAddr := "unix://" + allocdir.TaskGRPCSocket

	fn := filepath.Join(req.TaskDir.LocalDir, "bootstrap.json")
	id := agentconsul.MakeTaskServiceID(h.alloc.ID, tg.Name, service, canary)
	h.logger.Debug("bootstrapping envoy", "sidecar_for", service.Name, "boostrap_file", fn, "sidecar_for_id", id, "grpc_addr", grpcAddr)

	tries := 3

	// Before running bootstrap command, ensure service has been registered
	//TODO(schmichael) well this is one way to do it
RETRY:
	tries--

	//TODO(schmichael) run via docker container instead of host?
	cmd := exec.CommandContext(ctx, "consul", "connect", "envoy",
		"-grpc-addr", grpcAddr,
		"-http-addr", h.consulHTTPAddr,
		"-bootstrap",
		"-sidecar-for", id, // must use the id not the name!
	)

	// Redirect output to local/bootstrap.json
	fd, err := os.Create(fn)
	if err != nil {
		return fmt.Errorf("error creating local/bootstrap.json for envoy: %v", err)
	}
	cmd.Stdout = fd

	//TODO(schmichael) handle stderr better
	buf := bytes.NewBuffer(nil)
	cmd.Stderr = buf

	// Generate bootstrap
	err = cmd.Run()

	// Close stdout/bootstrap.json
	fd.Close()

	//TODO(schmichael) Remove
	contents, _ := ioutil.ReadFile(fn)
	stderr := buf.String()
	h.logger.Info("envoy bootstrap.json", "fn", fn, "json", string(contents), "stderr", stderr)

	// Check for error from command
	if err != nil {
		if tries > 0 {
			time.Sleep(3 * time.Second)
			goto RETRY
		}
		return fmt.Errorf("error creating bootstrap.json for envoy: %v", err)
	}

	// Bootstrap written. Mark as done and move on.
	resp.Done = true
	return nil
}
