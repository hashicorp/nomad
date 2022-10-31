package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
)

type apiHook struct {
	srv    config.APIListenerRegistrar
	logger log.Logger
	ln     net.Listener
}

func newAPIHook(srv config.APIListenerRegistrar, logger log.Logger) *apiHook {
	h := &apiHook{
		srv: srv,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*apiHook) Name() string {
	return "api"
}

func (h *apiHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	if h.ln != nil {
		//TODO(schmichael) remove me
		h.logger.Trace("huh, called apiHook.Prestart twice")
		return nil
	}

	//TODO(schmichael) what dir & perms
	udsDir := filepath.Join(req.TaskDir.SecretsDir, "run")
	if err := os.MkdirAll(udsDir, 0o775); err != nil {
		return fmt.Errorf("error creating api socket directory: %w", err)
	}

	//TODO(schmichael) what name
	udsPath := filepath.Join(udsDir, "nomad.socket")
	if err := os.RemoveAll(udsPath); err != nil {
		return fmt.Errorf("could not remove existing api socket: %w", err)
	}

	udsln, err := net.Listen("unix", udsPath)
	if err != nil {
		return fmt.Errorf("could not create api socket: %w", err)
	}

	go func() {
		if err := h.srv.Serve(ctx, udsln); err != nil {
			//TODO(schmichael) probably ignore http.ErrServerClosed
			h.logger.Warn("error serving api", "error", err)
		}
	}()

	h.ln = udsln
	return nil
}

func (h *apiHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	if h.ln == nil {
		//TODO(schmichael) remove me
		h.logger.Trace("huh, no listener")
		return nil
	}

	if err := h.ln.Close(); err != nil {
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		h.logger.Trace("error closing task listener: %v", err)
	}
	return nil
}
