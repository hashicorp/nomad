// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/time/rate"
)

const (
	// configEntriesRequestRateLimit is the maximum number of requests per second
	// Nomad will make against Consul for operations on global Configuration Entry
	// objects.
	configEntriesRequestRateLimit rate.Limit = 10
)

// ConsulConfigsAPI is an abstraction over the consul/api.ConfigEntries API used by
// Nomad Server.
//
// Nomad will only perform write operations on Consul Ingress/Terminating Gateway
// Configuration Entries. Removing the entries is not yet safe, given that multiple
// Nomad clusters may be writing to the same config entries, which are global in
// the Consul scope. There was a Meta field introduced which Nomad can leverage
// in the future, when Consul no longer supports versions that do not contain the
// field. The Meta field would be used to track which Nomad "owns" the CE.
// https://github.com/hashicorp/nomad/issues/8971
type ConsulConfigsAPI interface {
	// SetIngressCE adds the given ConfigEntry to Consul, overwriting
	// the previous entry if set.
	SetIngressCE(ctx context.Context, namespace, service, cluster, partition string, entry *structs.ConsulIngressConfigEntry) error

	// SetTerminatingCE adds the given ConfigEntry to Consul, overwriting
	// the previous entry if set.
	SetTerminatingCE(ctx context.Context, namespace, service, cluster, partition string, entry *structs.ConsulTerminatingConfigEntry) error

	// Stop is used to stop additional creations of Configuration Entries. Intended to
	// be used on Nomad Server shutdown.
	Stop()
}

type consulConfigsAPI struct {
	// configsClientFunc returns an interface that is the API subset of the real
	// Consul client we need for managing Configuration Entries.
	configsClientFunc consul.ConfigAPIFunc

	// limiter is used to rate limit requests to Consul
	limiter *rate.Limiter

	// logger is used to log messages
	logger hclog.Logger

	// lock protects the stopped flag, which prevents use of the consul configs API
	// client after shutdown.
	lock    sync.Mutex
	stopped bool
}

func NewConsulConfigsAPI(configsClientFunc consul.ConfigAPIFunc, logger hclog.Logger) *consulConfigsAPI {
	return &consulConfigsAPI{
		configsClientFunc: configsClientFunc,
		limiter:           rate.NewLimiter(configEntriesRequestRateLimit, int(configEntriesRequestRateLimit)),
		logger:            logger,
	}
}

func (c *consulConfigsAPI) Stop() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.stopped = true
}

func (c *consulConfigsAPI) SetIngressCE(ctx context.Context, namespace, service, cluster, partition string, entry *structs.ConsulIngressConfigEntry) error {
	return c.setCE(ctx, convertIngressCE(namespace, service, entry), cluster, partition)
}

func (c *consulConfigsAPI) SetTerminatingCE(ctx context.Context, namespace, service, cluster, partition string, entry *structs.ConsulTerminatingConfigEntry) error {
	return c.setCE(ctx, convertTerminatingCE(namespace, service, entry), cluster, partition)
}

// setCE will set the Configuration Entry of any type Consul supports.
func (c *consulConfigsAPI) setCE(ctx context.Context, entry api.ConfigEntry, cluster, partition string) error {
	defer metrics.MeasureSince([]string{"nomad", "consul", "create_config_entry"}, time.Now())

	// make sure the background deletion goroutine has not been stopped
	c.lock.Lock()
	stopped := c.stopped
	c.lock.Unlock()

	if stopped {
		return errors.New("client stopped and may not longer create config entries")
	}

	// ensure we are under our wait limit
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}

	client := c.configsClientFunc(cluster)
	_, _, err := client.Set(entry, &api.WriteOptions{
		Namespace: entry.GetNamespace(),
		Partition: partition,
	})
	return err
}

func convertIngressCE(namespace, service string, entry *structs.ConsulIngressConfigEntry) api.ConfigEntry {
	var listeners []api.IngressListener = nil
	for _, listener := range entry.Listeners {
		var services []api.IngressService = nil
		for _, s := range listener.Services {
			var sds *api.GatewayTLSSDSConfig = nil
			if s.TLS != nil {
				sds = convertGatewayTLSSDSConfig(s.TLS.SDS)
			}
			services = append(services, api.IngressService{
				Name:                  s.Name,
				Hosts:                 slices.Clone(s.Hosts),
				RequestHeaders:        convertHTTPHeaderModifiers(s.RequestHeaders),
				ResponseHeaders:       convertHTTPHeaderModifiers(s.ResponseHeaders),
				MaxConnections:        s.MaxConnections,
				MaxPendingRequests:    s.MaxPendingRequests,
				MaxConcurrentRequests: s.MaxConcurrentRequests,
				TLS: &api.GatewayServiceTLSConfig{
					SDS: sds,
				},
			})
		}
		listeners = append(listeners, api.IngressListener{
			Port:     listener.Port,
			Protocol: listener.Protocol,
			Services: services,
		})
	}

	tls := api.GatewayTLSConfig{}
	if entry.TLS != nil {
		tls.Enabled = entry.TLS.Enabled
		tls.TLSMinVersion = entry.TLS.TLSMinVersion
		tls.TLSMaxVersion = entry.TLS.TLSMaxVersion
		tls.CipherSuites = slices.Clone(entry.TLS.CipherSuites)
	}

	return &api.IngressGatewayConfigEntry{
		Namespace: namespace,
		Kind:      api.IngressGateway,
		Name:      service,
		TLS:       *convertGatewayTLSConfig(entry.TLS),
		Listeners: listeners,
	}
}

func convertHTTPHeaderModifiers(in *structs.ConsulHTTPHeaderModifiers) *api.HTTPHeaderModifiers {
	if in != nil {
		return &api.HTTPHeaderModifiers{
			Add:    maps.Clone(in.Add),
			Set:    maps.Clone(in.Set),
			Remove: slices.Clone(in.Remove),
		}
	}

	return &api.HTTPHeaderModifiers{}
}

func convertGatewayTLSConfig(in *structs.ConsulGatewayTLSConfig) *api.GatewayTLSConfig {
	if in != nil {
		return &api.GatewayTLSConfig{
			Enabled:       in.Enabled,
			TLSMinVersion: in.TLSMinVersion,
			TLSMaxVersion: in.TLSMaxVersion,
			CipherSuites:  slices.Clone(in.CipherSuites),
			SDS:           convertGatewayTLSSDSConfig(in.SDS),
		}
	}

	return &api.GatewayTLSConfig{}
}

func convertGatewayTLSSDSConfig(in *structs.ConsulGatewayTLSSDSConfig) *api.GatewayTLSSDSConfig {
	if in != nil {
		return &api.GatewayTLSSDSConfig{
			ClusterName:  in.ClusterName,
			CertResource: in.CertResource,
		}
	}

	return &api.GatewayTLSSDSConfig{}
}

func convertTerminatingCE(namespace, service string, entry *structs.ConsulTerminatingConfigEntry) api.ConfigEntry {
	var linked []api.LinkedService = nil
	for _, s := range entry.Services {
		linked = append(linked, api.LinkedService{
			Name:     s.Name,
			CAFile:   s.CAFile,
			CertFile: s.CertFile,
			KeyFile:  s.KeyFile,
			SNI:      s.SNI,
		})
	}
	return &api.TerminatingGatewayConfigEntry{
		Namespace: namespace,
		Kind:      api.TerminatingGateway,
		Name:      service,
		Services:  linked,
	}
}
