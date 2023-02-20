package taskrunner

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/nomad/structs"
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

	return h.setTlsFiles(ctx, req.TaskResources)
}

func (h *tlsHook) Update(ctx context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	return h.setTlsFiles(ctx, req.TaskResources)
}

// setTlsFiles adds the TLS files to the task's environment and writes it to a
// file if requested by the jobsepc.
func (h *tlsHook) setTlsFiles(ctx context.Context, resources *structs.AllocatedTaskResources) error {

	// TODO: Somehow get the key files here!
	// THIS IS HOW THE SIGNED IDENTITIES ARE FETCHED

	// token := h.tr.alloc.SignedIdentities[h.taskName]
	// if token == "" {
	// 	return nil
	// }

	// TODO: THIS WILL COME FROM A CERT THAT IS STORED IN STATE

	privateKeyFile := "/Users/mike/Code/nomad/nomad-agent-ca-key.pem"
	caPrivateKey, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return fmt.Errorf("Error reading CA priv key: %w", err)
	}

	pubKeyFile := "/Users/mike/Code/nomad/nomad-agent-ca.pem"
	caPubKey, err := os.ReadFile(pubKeyFile)
	if err != nil {
		return fmt.Errorf("Error reading CA pub key: %w", err)
	}

	signer, err := tlsutil.ParseSigner(string(caPrivateKey))
	if err != nil {
		return fmt.Errorf("failed to Parse signer: %w", err)
	}

	// TODO: What name to give it?
	name := "some-name!"

	var DNSNames []string
	DNSNames = append(DNSNames, "localhost")
	for _, network := range resources.Networks {
		DNSNames = append(DNSNames, network.Hostname)
	}

	var IPAddresses []net.IP
	IPAddresses = append(IPAddresses, net.ParseIP("127.0.0.1"))
	for _, network := range resources.Networks {
		IPAddresses = append(IPAddresses, net.ParseIP(network.IP))
	}

	// TODO: what does this do?
	var extKeyUsage []x509.ExtKeyUsage

	// pub, priv, err
	tlsCert, _, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer: signer, CA: string(caPubKey), Name: name, Days: 365,
		DNSNames: DNSNames, IPAddresses: IPAddresses, ExtKeyUsage: extKeyUsage,
	})
	if err != nil {
		return fmt.Errorf("failed to Generate cert: %w", err)
	}

	h.tr.setTlsValues(tlsCert, string(caPubKey))

	// TODO: Make this optional like in the identity hook
	if err := h.writeTlsValues(tlsCert, string(caPubKey)); err != nil {
		return fmt.Errorf("failed to write TLS values: %w", err)
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
