package taskrunner

import (
	"context"
	"sync"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
)

// identityHook sets the task runner's Nomad workload identity token
// based on the signed identity stored on the Allocation
type identityHook struct {
	tr       *TaskRunner
	logger   log.Logger
	taskName string
	lock     sync.Mutex
}

func newIdentityHook(tr *TaskRunner, logger log.Logger) *identityHook {
	h := &identityHook{
		tr:       tr,
		taskName: tr.taskName,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*identityHook) Name() string {
	return "identity"
}

func (h *identityHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	token := h.tr.alloc.SignedIdentities[h.taskName]
	if token != "" {
		h.tr.setNomadToken(token)
	}
	return nil
}
