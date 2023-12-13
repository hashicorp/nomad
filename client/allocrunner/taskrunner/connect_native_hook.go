// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	ifs "github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

const (
	connectNativeHookName = "connect_native"
)

type connectNativeHookConfig struct {
	consulShareTLS bool
	consul         consulTransportConfig
	alloc          *structs.Allocation
	logger         hclog.Logger
}

func newConnectNativeHookConfig(alloc *structs.Allocation, consul *config.ConsulConfig, logger hclog.Logger) *connectNativeHookConfig {
	return &connectNativeHookConfig{
		alloc:          alloc,
		logger:         logger,
		consulShareTLS: consul.ShareSSL == nil || *consul.ShareSSL, // default enabled
		consul:         newConsulTransportConfig(consul),
	}
}

// connectNativeHook manages additional automagic configuration for a connect
// native task.
//
// If nomad client is configured to talk to Consul using TLS (or other special
// auth), the native task will inherit that configuration EXCEPT for the consul
// token.
//
// If consul is configured with ACLs enabled, a Service Identity token will be
// generated on behalf of the native service and supplied to the task.
//
// If the alloc is configured with bridge networking enabled, the standard
// CONSUL_HTTP_ADDR environment variable is defaulted to the unix socket created
// for the alloc by the consul_grpc_sock_hook alloc runner hook.
type connectNativeHook struct {
	// alloc is the allocation with the connect native task being run
	alloc *structs.Allocation

	// consulShareTLS is used to toggle whether the TLS configuration of the
	// Nomad Client may be shared with Connect Native applications.
	consulShareTLS bool

	// consulConfig is used to enable the connect native enabled task to
	// communicate with consul directly, as is necessary for the task to request
	// its connect mTLS certificates.
	consulConfig consulTransportConfig

	// logger is used to log things
	logger hclog.Logger
}

func newConnectNativeHook(c *connectNativeHookConfig) *connectNativeHook {
	return &connectNativeHook{
		alloc:          c.alloc,
		consulShareTLS: c.consulShareTLS,
		consulConfig:   c.consul,
		logger:         c.logger.Named(connectNativeHookName),
	}
}

func (connectNativeHook) Name() string {
	return connectNativeHookName
}

// merge b into a, overwriting on conflicts
func merge(a, b map[string]string) {
	for k, v := range b {
		a[k] = v
	}
}

func (h *connectNativeHook) Prestart(
	ctx context.Context,
	request *ifs.TaskPrestartRequest,
	response *ifs.TaskPrestartResponse) error {

	if !request.Task.Kind.IsConnectNative() {
		response.Done = true
		return nil
	}

	environment := make(map[string]string)

	if h.consulShareTLS {
		// copy TLS certificates
		if err := h.copyCertificates(h.consulConfig, request.TaskDir.SecretsDir); err != nil {
			h.logger.Error("failed to copy Consul TLS certificates", "error", err)
			return err
		}

		// set environment variables for communicating with Consul agent, but
		// only if those environment variables are not already set
		merge(environment, h.tlsEnv(request.TaskEnv.EnvMap))
	}

	if err := h.maybeSetSITokenEnv(request.TaskDir.SecretsDir, request.Task.Name, environment); err != nil {
		h.logger.Error("failed to load Consul Service Identity Token", "error", err, "task", request.Task.Name)
		return err
	}

	merge(environment, h.bridgeEnv(request.TaskEnv.EnvMap))
	merge(environment, h.hostEnv(request.TaskEnv.EnvMap))

	// tls/acl setup for native task done
	response.Done = true
	response.Env = environment
	return nil
}

const (
	secretCAFilename       = "consul_ca_file.pem"
	secretCertfileFilename = "consul_cert_file.pem"
	secretKeyfileFilename  = "consul_key_file.pem"
)

func (h *connectNativeHook) copyCertificates(consulConfig consulTransportConfig, dir string) error {
	if err := h.copyCertificate(consulConfig.CAFile, dir, secretCAFilename); err != nil {
		return err
	}
	if err := h.copyCertificate(consulConfig.CertFile, dir, secretCertfileFilename); err != nil {
		return err
	}
	if err := h.copyCertificate(consulConfig.KeyFile, dir, secretKeyfileFilename); err != nil {
		return err
	}
	return nil
}

func (connectNativeHook) copyCertificate(source, dir, name string) error {
	if source == "" {
		return nil
	}

	original, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open consul TLS certificate: %w", err)
	}
	defer original.Close()

	destination := filepath.Join(dir, name)
	fd, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create secrets/%s: %w", name, err)
	}
	defer fd.Close()

	if _, err := io.Copy(fd, original); err != nil {
		return fmt.Errorf("failed to copy certificate secrets/%s: %w", name, err)
	}

	if err := fd.Sync(); err != nil {
		return fmt.Errorf("failed to write secrets/%s: %w", name, err)
	}

	return nil
}

// tlsEnv creates a set of additional of environment variables to be used when launching
// the connect native task. This will enable the task to communicate with Consul
// if Consul has transport security turned on.
//
// We do NOT set CONSUL_HTTP_TOKEN from the nomad agent's consul config, as that
// is a separate security concern addressed by the service identity hook.
func (h *connectNativeHook) tlsEnv(env map[string]string) map[string]string {
	m := make(map[string]string)

	if _, exists := env["CONSUL_CACERT"]; !exists && h.consulConfig.CAFile != "" {
		m["CONSUL_CACERT"] = filepath.Join("/secrets", secretCAFilename)
	}

	if _, exists := env["CONSUL_CLIENT_CERT"]; !exists && h.consulConfig.CertFile != "" {
		m["CONSUL_CLIENT_CERT"] = filepath.Join("/secrets", secretCertfileFilename)
	}

	if _, exists := env["CONSUL_CLIENT_KEY"]; !exists && h.consulConfig.KeyFile != "" {
		m["CONSUL_CLIENT_KEY"] = filepath.Join("/secrets", secretKeyfileFilename)
	}

	if _, exists := env["CONSUL_HTTP_SSL"]; !exists {
		if v := h.consulConfig.SSL; v != "" {
			m["CONSUL_HTTP_SSL"] = v
		}
	}

	if _, exists := env["CONSUL_HTTP_SSL_VERIFY"]; !exists {
		if v := h.consulConfig.VerifySSL; v != "" {
			m["CONSUL_HTTP_SSL_VERIFY"] = v
		}
	}

	return m
}

// bridgeEnv creates a set of additional environment variables to be used when launching
// the connect native task. This will enable the task to communicate with Consul
// if the task is running inside an alloc's network namespace (i.e. bridge mode).
//
// Sets CONSUL_HTTP_ADDR if not already set.
// Sets CONSUL_TLS_SERVER_NAME if not already set, and consul tls is enabled.
func (h *connectNativeHook) bridgeEnv(env map[string]string) map[string]string {

	if h.alloc.AllocatedResources.Shared.Networks[0].Mode != "bridge" {
		return nil
	}

	result := make(map[string]string)

	if _, exists := env["CONSUL_HTTP_ADDR"]; !exists {
		result["CONSUL_HTTP_ADDR"] = "unix:///" + allocdir.AllocHTTPSocket
	}

	if _, exists := env["CONSUL_TLS_SERVER_NAME"]; !exists {
		if v := h.consulConfig.SSL; v != "" {
			result["CONSUL_TLS_SERVER_NAME"] = "localhost"
		}
	}

	return result
}

// hostEnv creates a set of additional environment variables to be used when launching
// the connect native task. This will enable the task to communicate with Consul
// if the task is running in host network mode.
//
// Sets CONSUL_HTTP_ADDR if not already set.
func (h *connectNativeHook) hostEnv(env map[string]string) map[string]string {
	if h.alloc.AllocatedResources.Shared.Networks[0].Mode != "host" {
		return nil
	}

	if _, exists := env["CONSUL_HTTP_ADDR"]; !exists {
		return map[string]string{
			"CONSUL_HTTP_ADDR": h.consulConfig.HTTPAddr,
		}
	}

	return nil
}

// maybeSetSITokenEnv will set the CONSUL_HTTP_TOKEN environment variable in
// the given env map, if the token is found to exist in the task's secrets
// directory AND the CONSUL_HTTP_TOKEN environment variable is not already set.
//
// Following the pattern of the envoy_bootstrap_hook, the Consul Service Identity
// ACL Token is generated prior to this hook, if Consul ACLs are enabled. This is
// done in the sids_hook, which places the token at secrets/si_token in the task
// workspace. The content of that file is the SI token specific to this task
// instance.
func (h *connectNativeHook) maybeSetSITokenEnv(dir, task string, env map[string]string) error {
	if _, exists := env["CONSUL_HTTP_TOKEN"]; exists {
		// Consul token was already set - typically by using the Vault integration
		// and a template block to set the environment. Ignore the SI token as
		// the configured token takes precedence.
		return nil
	}

	token, err := os.ReadFile(filepath.Join(dir, sidsTokenFile))
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to load SI token for native task %s: %w", task, err)
		}
		h.logger.Trace("no SI token to load for native task", "task", task)
		return nil // token file DNE; acls not enabled
	}
	h.logger.Trace("recovered pre-existing SI token for native task", "task", task)
	env["CONSUL_HTTP_TOKEN"] = string(token)
	return nil
}
