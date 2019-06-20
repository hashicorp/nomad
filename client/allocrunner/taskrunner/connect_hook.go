package taskrunner

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

var _ interfaces.TaskPrestartHook = &connectHook{}

// connectHook writes the bootstrap config for the envoy sidecar proxy
type connectHook struct {
	alloc        *structs.Allocation
	consulClient consul.ConsulServiceAPI

	logger log.Logger
}

func newConnectHook(logger log.Logger, alloc *structs.Allocation, consulClient consul.ConsulServiceAPI) *connectHook {
	h := &connectHook{
		alloc:        alloc,
		consulClient: consulClient,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (connectHook) Name() string {
	return "connect"
}

func (h *connectHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	//TODO(schmichael) this is a pretty silly way of only running for one task
	if req.Task.Name != "_envoy" {
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

	h.logger.Debug("bootstrapping envoy", "sidecar_for", service.Name)

	// Before running bootstrap command, ensure service has been registered
	//TODO(schmichael)

	//TODO(schmichael) run via docker container instead of host?
	cmd := exec.CommandContext(ctx, "consul", "connect", "envoy",
		"-bootstrap",
		"-sidecar-for", service.Name,
	)

	// Redirect output to local/bootstrap.json
	fn := filepath.Join(req.TaskDir.LocalDir, "bootstrap.json")
	fd, err := os.Create(fn)
	if err != nil {
		return fmt.Errorf("error creating local/bootstrap.json for envoy: %v", err)
	}
	cmd.Stdout = fd

	//TODO(schmichael) override stderr
	//cmd.Stderr =

	// Generate bootstrap
	err = cmd.Run()

	// Close stdout/bootstrap.json
	fd.Close()

	//TODO(schmichael) Remove
	contents, _ := ioutil.ReadFile(fn)
	h.logger.Info("envoy bootstrap.json", "fn", fn, "json", contents)

	// Check for error from command
	if err != nil {
		return fmt.Errorf("error creating bootstrap.json for envoy: %v", err)
	}

	// Bootstrap written. Mark as done and move on.
	resp.Done = true
	return nil
}
