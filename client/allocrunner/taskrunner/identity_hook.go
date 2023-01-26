package taskrunner

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
)

// identityHook sets the task runner's Nomad workload identity token
// based on the signed identity stored on the Allocation

const (
	// wiTokenFile is the name of the file holding the Nomad token inside the
	// task's secret directory
	wiTokenFile = "nomad_token"
)

type identityHook struct {
	tr       *TaskRunner
	logger   log.Logger
	taskName string
	lock     sync.Mutex

	// tokenPath is the path in which to read and write the token
	tokenPath string
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
	h.tokenPath = filepath.Join(req.TaskDir.SecretsDir, wiTokenFile)

	token := h.tr.alloc.SignedIdentities[h.taskName]
	if token == "" {
		return nil
	}
	h.tr.setNomadToken(token)
	if h.tr.task.EmitWorkloadToken {
		h.writeToken(token)
	}

	return nil
}

func (h *identityHook) Update(_ context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	token := h.tr.alloc.SignedIdentities[h.taskName]
	if token == "" {
		return nil
	}
	h.tr.setNomadToken(token)
	if h.tr.task.EmitWorkloadToken {
		h.writeToken(token)
	}

	return nil
}

// writeToken writes the given token to disk
func (h *identityHook) writeToken(token string) error {
	if err := ioutil.WriteFile(h.tokenPath, []byte(token), 0666); err != nil {
		return fmt.Errorf("failed to write nomad token: %v", err)
	}

	return nil
}
