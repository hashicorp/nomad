package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
)

type apiHook struct {
	srv    config.APIListenerRegistrar
	logger hclog.Logger
	ln     net.Listener
}

func newAPIHook(srv config.APIListenerRegistrar, logger hclog.Logger) *apiHook {
	h := &apiHook{
		srv: srv,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*apiHook) Name() string {
	return "api"
}

func (h *apiHook) Prestart(_ context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	udsDir := filepath.Join(req.TaskDir.SecretsDir, "run")
	if err := os.MkdirAll(udsDir, 0o775); err != nil {
		return fmt.Errorf("error creating api socket directory: %w", err)
	}

	udsPath := filepath.Join(udsDir, "nomad.socket")
	if err := os.RemoveAll(udsPath); err != nil {
		return fmt.Errorf("could not remove existing api socket: %w", err)
	}

	udsln, err := net.Listen("unix", udsPath)
	if err != nil {
		return fmt.Errorf("could not create api socket: %w", err)
	}

	go func() {
		// Cannot use Prestart's context as it is closed after all prestart hooks
		// have been closed.
		if err := h.srv.Serve(context.TODO(), udsln); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			h.logger.Error("error serving api", "error", err)
		}
	}()

	h.ln = udsln
	return nil
}

func (h *apiHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	if h.ln != nil {
		if err := h.ln.Close(); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				h.logger.Trace("error closing task listener: %v", err)
			}
		}
	}

	// Best-effort at cleaining things up. Alloc dir cleanup will remove it if
	// this fails for any reason.
	udsPath := filepath.Join(req.TaskDir.SecretsDir, "run", "nomad.socket")
	_ = os.RemoveAll(udsPath)

	return nil
}
