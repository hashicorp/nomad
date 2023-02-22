package taskrunner

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
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

type setTlsOpts struct {
	Namespace string
	Region    string
	Resources *structs.AllocatedTaskResources
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

	opts := setTlsOpts{
		Namespace: req.Alloc.Namespace,
		Region:    req.Alloc.Job.Region,
		Resources: req.TaskResources,
	}

	return h.setTlsFiles(ctx, opts)
}

func (h *tlsHook) Update(ctx context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	opts := setTlsOpts{
		Namespace: req.Alloc.Namespace,
		Region:    req.Alloc.Job.Region,
		Resources: req.TaskResources,
	}

	return h.setTlsFiles(ctx, opts)
}

// setTlsFiles adds the TLS files to the task's environment and writes it to a
// file if requested by the jobsepc.
func (h *tlsHook) setTlsFiles(ctx context.Context, opts setTlsOpts) error {
	resources := opts.Resources
	// TODO: Somehow get the key files here!
	// THIS IS HOW THE SIGNED IDENTITIES ARE FETCHED

	// token := h.tr.alloc.SignedIdentities[h.taskName]
	// if token == "" {
	// 	return nil
	// }

	caPrivateKey, caPubKey, _ := h.getCaKeys(opts)

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

func (h *tlsHook) getCaKeys(opts setTlsOpts) (string, string, error) {
	path := "tls"

	args := structs.VariablesReadRequest{
		Path: path,
		QueryOptions: structs.QueryOptions{
			Namespace: opts.Namespace,
			Region:    opts.Region,
		},
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
		// fmt.Println("XKCD - NO DATA SO CREATING A CA")
		privateKey, publicKey, err := h.createNewCA(opts)
		if err != nil {
			return "", "", err
		}

		// May overwrite privateKey & publicKey if there is a conflict
		return h.writeCAToVariable(privateKey, publicKey, opts)
	} else {
		// fmt.Println("XKCD - DATA EXISTS, SO USING OLD CA")
		return h.useExistingCA(out, opts)
	}
}

func (h *tlsHook) createNewCA(opts setTlsOpts) (string, string, error) {
	caOpts := tlsutil.CAOpts{
		Name:                fmt.Sprintf("internal-nomad-ca-%s-%s", opts.Namespace, opts.Region),
		Days:                9999,
		Domain:              "nomad",
		PermittedDNSDomains: []string{},
	}

	publicKey, privateKey, err := tlsutil.GenerateCA(caOpts)
	if err != nil {
		return "", "", fmt.Errorf("Error generating new CA: %w", err)
	}

	return privateKey, publicKey, nil
}

func (h *tlsHook) useExistingCA(existingCAData structs.VariablesReadResponse, opts setTlsOpts) (string, string, error) {
	privateKeyFromVar, publicKeyFromVar, err := decodeKeys(existingCAData.Data)
	if err != nil {
		return "", "", err
	}

	if privateKeyFromVar != "" && publicKeyFromVar != "" {
		return privateKeyFromVar, publicKeyFromVar, nil
	}

	return "", "", errors.New("could not get or generate CA keys")
}

func (h *tlsHook) writeCAToVariable(privateKey, publicKey string, opts setTlsOpts) (string, string, error) {
	var Variable structs.VariableDecrypted
	Variable.Path = "tls"
	Variable.Items = structs.VariableItems{
		"private-key": base64.StdEncoding.EncodeToString([]byte(privateKey)),
		"public-key":  base64.StdEncoding.EncodeToString([]byte(publicKey)),
	}
	Variable.ModifyIndex = 0

	args := structs.VariablesApplyRequest{
		Op:  structs.VarOpCAS,
		Var: &Variable,
		WriteRequest: structs.WriteRequest{
			Region:    opts.Region,
			Namespace: opts.Namespace,
		},
	}

	// fmt.Println("XKCD - ATTEMPTING WRITE")
	var out structs.VariablesApplyResponse
	if err := h.rpcer.RPC(structs.VariablesApplyRPCMethod, &args, &out); err != nil {
		if strings.Contains(err.Error(), "cas error:") && out.Conflict != nil {
			// fmt.Println("XKCD - HAS CONFLICT BUT NO OUTPUT")
			return "", "", fmt.Errorf("conflicting value: %w", err)
		}

		// TODO: I THINK THERE IS A BUG HERE WITH THE CONDITIONS WHERE THIS IS HIT
		if out.Conflict != nil {
			// fmt.Println("XKCD - USING THE CONFLICT VALUE")
			return decodeKeys(out.Conflict)
		}

		// fmt.Println("XKCD - SOME ERROR")
		return "", "", fmt.Errorf("some write error: %w", err)
	}

	if out.Conflict != nil {
		// fmt.Println("XKCD - USING THE CONFLICT VALUE - NO ERROR")
		return decodeKeys(out.Conflict)
	}

	// fmt.Println("XKCD - PROPERLY WRITTEN AND USING!")
	return decodeKeys(out.Output)
}

func decodeKeys(variableData *structs.VariableDecrypted) (string, string, error) {
	privateKeyFromVarBase64 := variableData.Items["private-key"]
	privateKeyFromVarBytes, err := base64.StdEncoding.DecodeString(privateKeyFromVarBase64)
	if err != nil {
		return "", "", fmt.Errorf("Error decoding base64 private CA key: %w", err)
	}

	publicKeyFromVarBase64 := variableData.Items["public-key"]
	publicKeyFromVarBytes, err := base64.StdEncoding.DecodeString(publicKeyFromVarBase64)
	if err != nil {
		return "", "", fmt.Errorf("Error decoding base64 public CA key: %w", err)
	}

	privateKeyFromVar := string(privateKeyFromVarBytes)
	publicKeyFromVar := string(publicKeyFromVarBytes)

	return privateKeyFromVar, publicKeyFromVar, nil
}
