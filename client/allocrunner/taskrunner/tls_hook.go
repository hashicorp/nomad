package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/users"
)

// tlsHook sets the task runner's TLS cert and CA public key

const (
	// tlsCertFile is the name of the file holding the TLS cert for the alloc
	tlsCertFile = "nomad_tls_cert"
	// tlsCAPubKeyFile is the name of the file holding the CA's public key
	tlsCAPubKeyFile = "nomad_tls_ca_key"
)

type tlsHook struct {
	tr       *TaskRunner
	logger   log.Logger
	taskName string
	lock     sync.Mutex

	// certPath is the path in which to read and write the cert
	certPath string

	// caPath is the path in which to read and write the ca public key
	caPath string
}

func newTlsHook(tr *TaskRunner, logger log.Logger) *tlsHook {
	h := &tlsHook{
		tr:       tr,
		taskName: tr.taskName,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*tlsHook) Name() string {
	return "tls"
}

func (h *tlsHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.certPath = filepath.Join(req.TaskDir.SecretsDir, tlsCertFile)
	h.caPath = filepath.Join(req.TaskDir.SecretsDir, tlsCAPubKeyFile)

	return h.setTlsFiles()
}

func (h *tlsHook) Update(_ context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	return h.setTlsFiles()
}

// setTlsFiles adds the TLS files to the task's environment and writes it to a
// file if requested by the jobsepc.
func (h *tlsHook) setTlsFiles() error {

	// TODO: Somehow get the files here!
	// token := h.tr.alloc.SignedIdentities[h.taskName]
	// if token == "" {
	// 	return nil
	// }

	tlsCert := "THIS IS THE TLS CERT"
	caKey := "THIS IS THE CA KEY"

	h.tr.setTlsValues(tlsCert, caKey)

	// TODO: Make this optional like in the identity hook
	if err := h.writeTlsValues(tlsCert, caKey); err != nil {
		return err
	}

	return nil
}

// writeToken writes the given token to disk
func (h *tlsHook) writeTlsValues(tlsCert, caKey string) error {
	// Write token as owner readable only
	if err := users.WriteFileFor(h.certPath, []byte(tlsCert), h.tr.task.User); err != nil {
		return fmt.Errorf("failed to write TLS cert: %w", err)
	}

	if err := users.WriteFileFor(h.caPath, []byte(caKey), h.tr.task.User); err != nil {
		return fmt.Errorf("failed to write CA public key: %w", err)
	}

	return nil
}
