package taskrunner

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
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
	tlsKeyDir = "tls_keys"
	caCertDir = "ca_certs"
	// tlsCertPubKeyFile is the name of the file holding the TLS cert public key
	tlsCertPubKeyFile = "public_key.pem"
	// tlsCertPrivKeyFile is the name of the file holding the TLS cert private key
	tlsCertPrivKeyFile = "private_key.pem"
	// tlsCAPubKeyFileSuffix is the name of the file holding the CA's public key
	tlsCAPubKeyFileSuffix = "ca_public_key.pem"
)

type tlsHook struct {
	tr       *TaskRunner
	logger   log.Logger
	taskName string
	lock     sync.Mutex

	secretsDir string

	rpcer RPCer
}

type setTlsOpts struct {
	Namespace    string
	Region       string
	Resources    *structs.AllocatedTaskResources
	TrustCircles []string
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
	h.secretsDir = req.TaskDir.SecretsDir

	opts := setTlsOpts{
		Namespace:    req.Alloc.Namespace,
		Region:       req.Alloc.Job.Region,
		Resources:    req.TaskResources,
		TrustCircles: req.Task.TrustCircles,
	}

	return h.setTlsFiles(ctx, opts)
}

func (h *tlsHook) Update(ctx context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	opts := setTlsOpts{
		Namespace:    req.Alloc.Namespace,
		Region:       req.Alloc.Job.Region,
		Resources:    req.TaskResources,
		TrustCircles: req.Task.TrustCircles,
	}

	return h.setTlsFiles(ctx, opts)
}

// setTlsFiles adds the TLS files to the task's environment and writes it to a
// file if requested by the jobsepc.
func (h *tlsHook) setTlsFiles(ctx context.Context, opts setTlsOpts) error {

	// Set up the tls for the namespace
	err := h.setUpNamespaceTls(ctx, opts)
	if err != nil {
		return err
	}

	// Set up tls for any declared circles
	for _, circleName := range opts.TrustCircles {
		err = h.setUpCircleTls(ctx, circleName, opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *tlsHook) setUpCircleTls(ctx context.Context, circleName string, opts setTlsOpts) error {
	resources := opts.Resources
	variablePath := fmt.Sprintf("tls/cot/%s", circleName)
	dashName := fmt.Sprintf("cot-%s", circleName)
	caPrivateKey, caPubKey, err := h.getCaKeys(variablePath, dashName, true, opts)
	if err != nil {
		return fmt.Errorf("error getting ca keys %w", err)
	}
	tlsPublicCert, tlsPrivateCert, err := createTlsCerts(caPrivateKey, caPubKey, resources)
	if err != nil {
		return fmt.Errorf("error generating tls certs %w", err)
	}

	// ???not sure if I need this???
	// h.tr.setTlsValues(tlsPublicCert, tlsPrivateCert, caPubKey, opts.TrustCircles)

	// TODO: CHECK THIS
	if err := h.writeTlsValues(dashName, tlsPublicCert, tlsPrivateCert, caPubKey); err != nil {
		return fmt.Errorf("failed to write TLS values: %w", err)
	}

	return nil
}

func (h *tlsHook) setUpNamespaceTls(ctx context.Context, opts setTlsOpts) error {
	resources := opts.Resources
	variablePath := "tls/ns"
	dashName := "ns"
	caPrivateKey, caPubKey, err := h.getCaKeys(variablePath, dashName, false, opts)
	if err != nil {
		return fmt.Errorf("error getting ca keys %w", err)
	}
	tlsPublicCert, tlsPrivateCert, err := createTlsCerts(caPrivateKey, caPubKey, resources)
	if err != nil {
		return fmt.Errorf("error generating tls certs %w", err)
	}

	h.tr.setTlsValues(tlsPublicCert, tlsPrivateCert, caPubKey, opts.TrustCircles)

	// TODO: Make this optional with jobspec config (see identity hook)
	if err := h.writeTlsValues(dashName, tlsPublicCert, tlsPrivateCert, caPubKey); err != nil {
		return fmt.Errorf("failed to write TLS values: %w", err)
	}

	return nil
}

func createTlsCerts(caPrivateKey, caPubKey string, resources *structs.AllocatedTaskResources) (string, string, error) {
	signer, err := tlsutil.ParseSigner(caPrivateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to Parse signer: %w", err)
	}

	// TODO: Figure out what name this should have
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

	// TODO: Figure out what this does
	var extKeyUsage []x509.ExtKeyUsage

	tlsPublicCert, tlsPrivateCert, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer: signer, CA: caPubKey, Name: name, Days: 365,
		DNSNames: DNSNames, IPAddresses: IPAddresses, ExtKeyUsage: extKeyUsage,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to Generate cert: %w", err)
	}

	return tlsPublicCert, tlsPrivateCert, nil
}

// writeToken writes the given token to disk
func (h *tlsHook) writeTlsValues(namespaceOrCotName, tlsPublicCert, tlsPrivateCert, tlsCAPubKey string) error {
	namespacedTlsPath := filepath.Join(h.secretsDir, tlsKeyDir, namespaceOrCotName)
	pubKeyPath := filepath.Join(namespacedTlsPath, tlsCertPubKeyFile)
	privKeyPath := filepath.Join(namespacedTlsPath, tlsCertPrivKeyFile)

	tlsCaPubKeyName := fmt.Sprintf("%s_%s", namespaceOrCotName, tlsCAPubKeyFileSuffix)
	caCertDirPath := filepath.Join(h.secretsDir, caCertDir)
	caCertPath := filepath.Join(caCertDirPath, tlsCaPubKeyName)

	if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
		os.MkdirAll(namespacedTlsPath, 0700) // Create your file
	}
	if err := users.WriteFileFor(pubKeyPath, []byte(tlsPublicCert), h.tr.task.User); err != nil {
		return fmt.Errorf("failed to write TLS Pub cert: %w", err)
	}

	if err := users.WriteFileFor(privKeyPath, []byte(tlsPrivateCert), h.tr.task.User); err != nil {
		return fmt.Errorf("failed to write TLS Private cert: %w", err)
	}

	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		os.MkdirAll(caCertDirPath, 0700) // Create your file
	}
	if err := users.WriteFileFor(caCertPath, []byte(tlsCAPubKey), h.tr.task.User); err != nil {
		return fmt.Errorf("failed to write CA public key: %w", err)
	}

	return nil
}

func (h *tlsHook) getCaKeys(variablePath, dashName string, globalNs bool, opts setTlsOpts) (string, string, error) {
	namespace := opts.Namespace
	if globalNs {
		namespace = "default"
	}

	args := structs.VariablesReadRequest{
		Path: variablePath,
		QueryOptions: structs.QueryOptions{
			Namespace: namespace,
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
		privateKey, publicKey, err := h.createNewCA(dashName, opts)
		if err != nil {
			return "", "", err
		}

		// May overwrite privateKey & publicKey if there is a conflict
		return h.writeCAToVariable(variablePath, privateKey, publicKey, opts)
	} else {
		return h.useExistingCA(out, opts)
	}
}

func (h *tlsHook) createNewCA(nameSuffix string, opts setTlsOpts) (string, string, error) {
	caOpts := tlsutil.CAOpts{
		Name:                fmt.Sprintf("internal-nomad-ca-%s", nameSuffix),
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

func (h *tlsHook) writeCAToVariable(variablePath, privateKey, publicKey string, opts setTlsOpts) (string, string, error) {
	var Variable structs.VariableDecrypted
	Variable.Path = variablePath
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

	var out structs.VariablesApplyResponse
	if err := h.rpcer.RPC(structs.VariablesApplyRPCMethod, &args, &out); err != nil {
		if strings.Contains(err.Error(), "cas error:") && out.Conflict != nil {
			return "", "", fmt.Errorf("conflicting value: %w", err)
		}

		// TODO: I THINK THERE IS A BUG HERE WITH THE CONDITIONS WHERE THIS IS HIT
		if out.Conflict != nil {
			return decodeKeys(out.Conflict)
		}

		return "", "", fmt.Errorf("some write error: %w", err)
	}

	if out.Conflict != nil {
		return decodeKeys(out.Conflict)
	}

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
