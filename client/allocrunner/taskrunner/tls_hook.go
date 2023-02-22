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
	// tlsCertPubKeyFile is the name of the file holding the TLS cert public key
	tlsCertPubKeyFile = "nomad_tls_public_key.pem"
	// tlsCertPrviKeyFile is the name of the file holding the TLS cert private key
	tlsCertPrviKeyFile = "nomad_tls_private_key.pem"
	// tlsCAPubKeyFile is the name of the file holding the CA's public key
	tlsCAPubKeyFile = "nomad_tls_ca_public_key.pem"
)

type tlsHook struct {
	tr       *TaskRunner
	logger   log.Logger
	taskName string
	lock     sync.Mutex

	// certPrivKeyPath is the path in which to read and write the cert pub key
	certPrivKeyPath string

	// certPubKeyPath is the path in which to read and write the cert priv key
	certPubKeyPath string

	// caPubKeyPath is the path in which to read and write the ca public key
	caPubKeyPath string

	rpcer RPCer
}

func newTlsHook(tr *TaskRunner, rpcer RPCer, logger log.Logger) *tlsHook {
	h := &tlsHook{
		tr:       tr,
		taskName: tr.taskName,
		rpcer:    rpcer,
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
	h.certPubKeyPath = filepath.Join(req.TaskDir.SecretsDir, tlsCertPubKeyFile)
	h.certPrivKeyPath = filepath.Join(req.TaskDir.SecretsDir, tlsCertPrviKeyFile)
	h.caPubKeyPath = filepath.Join(req.TaskDir.SecretsDir, tlsCAPubKeyFile)

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

	path := "tls/testing"

	args := structs.VariablesReadRequest{
		Path: path,
	}
	var out structs.VariablesReadResponse

	err := h.rpcer.RPC(
		structs.VariablesReadRPCMethod,
		&args,
		&out,
	)

	if err != nil {
		panic(err)
	}

	if out.Data == nil {
		fmt.Println("XKCD - IT WAS NIL")
	} else {
		fmt.Println("XKCD - GOT DATA!!!")
	}

	caPrivateKey, caPubKey, _ := h.getCaKeys()

	signer, err := tlsutil.ParseSigner(caPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to Parse signer: %w", err)
	}

	// TODO: What name to give it?
	// Something from service disco?
	// Tho, I don't think this matters because DNSNames
	// & IPAddresses inform alt names
	name := "*"

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

	tlsPublicCert, tlsPrivateCert, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer: signer, CA: caPubKey, Name: name, Days: 365,
		DNSNames: DNSNames, IPAddresses: IPAddresses, ExtKeyUsage: extKeyUsage,
	})
	if err != nil {
		return fmt.Errorf("failed to Generate cert: %w", err)
	}

	h.tr.setTlsValues(tlsPublicCert, tlsPrivateCert, caPubKey)

	// TODO: Make this optional like in the identity hook
	if err := h.writeTlsValues(tlsPublicCert, tlsPrivateCert, caPubKey); err != nil {
		return fmt.Errorf("failed to write TLS values: %w", err)
	}

	return nil
}

// writeToken writes the given token to disk
func (h *tlsHook) writeTlsValues(tlsPublicCert, tlsPrivateCert, tlsCAPubKey string) error {
	// Write token as owner readable only
	if err := users.WriteFileFor(h.certPubKeyPath, []byte(tlsPublicCert), h.tr.task.User); err != nil {
		return fmt.Errorf("failed to write TLS Pub cert: %w", err)
	}

	if err := users.WriteFileFor(h.certPrivKeyPath, []byte(tlsPrivateCert), h.tr.task.User); err != nil {
		return fmt.Errorf("failed to write TLS Private cert: %w", err)
	}

	if err := users.WriteFileFor(h.caPubKeyPath, []byte(tlsCAPubKey), h.tr.task.User); err != nil {
		return fmt.Errorf("failed to write CA public key: %w", err)
	}

	return nil
}

func (h *tlsHook) getCaKeys() (string, string, error) {
	privateKeyFile := "/Users/mike/Code/nomad/nomad-agent-ca-key.pem"
	caPrivateKey, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return "", "", fmt.Errorf("Error reading CA priv key: %w", err)
	}

	pubKeyFile := "/Users/mike/Code/nomad/nomad-agent-ca.pem"
	caPubKey, err := os.ReadFile(pubKeyFile)
	if err != nil {
		return "", "", fmt.Errorf("Error reading CA pub key: %w", err)
	}

	return string(caPrivateKey), string(caPubKey), nil
}
