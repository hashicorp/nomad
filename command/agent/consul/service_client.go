// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"fmt"
	"maps"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/envoy"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// nomadServicePrefix is the prefix that scopes all Nomad registered
	// services (both agent and task entries).
	nomadServicePrefix = "_nomad"

	// nomadServerPrefix is the prefix that scopes Nomad registered Servers.
	nomadServerPrefix = nomadServicePrefix + "-server-"

	// nomadClientPrefix is the prefix that scopes Nomad registered Clients.
	nomadClientPrefix = nomadServicePrefix + "-client-"

	// nomadTaskPrefix is the prefix that scopes Nomad registered services
	// for tasks.
	nomadTaskPrefix = nomadServicePrefix + "-task-"

	// nomadCheckPrefix is the prefix that scopes Nomad registered checks for
	// services.
	nomadCheckPrefix = nomadServicePrefix + "-check-"

	// defaultRetryInterval is how quickly to retry syncing services and
	// checks to Consul when an error occurs. Will backoff up to a max.
	defaultRetryInterval = time.Second

	// defaultMaxRetryInterval is the default max retry interval.
	defaultMaxRetryInterval = 30 * time.Second

	// defaultPeriodicalInterval is the interval at which the service
	// client reconciles state between the desired services and checks and
	// what's actually registered in Consul. This is done at an interval,
	// rather than being purely edge triggered, to handle the case that the
	// Consul agent's state may change underneath us
	defaultPeriodicInterval = 30 * time.Second

	// ttlCheckBuffer is the time interval that Nomad can take to report Consul
	// the check result
	ttlCheckBuffer = 31 * time.Second

	// defaultShutdownWait is how long Shutdown() should block waiting for
	// enqueued operations to sync to Consul by default.
	defaultShutdownWait = time.Minute

	// DefaultQueryWaitDuration is the max duration the Consul Agent will
	// spend waiting for a response from a Consul Query.
	DefaultQueryWaitDuration = 2 * time.Second

	// ServiceTagHTTP is the tag assigned to HTTP services
	ServiceTagHTTP = "http"

	// ServiceTagRPC is the tag assigned to RPC services
	ServiceTagRPC = "rpc"

	// ServiceTagSerf is the tag assigned to Serf services
	ServiceTagSerf = "serf"

	// deregisterProbationPeriod is the initialization period where
	// services registered in Consul but not in Nomad don't get deregistered,
	// to allow for nomad restoring tasks
	deregisterProbationPeriod = time.Minute
)

// Additional Consul ACLs required
// - Consul Template: key:read
//   Used in tasks with template block that use Consul keys.

// CatalogAPI is the consul/api.Catalog API used by Nomad.
//
// ACL requirements
// - node:read (listing datacenters)
// - service:read
type CatalogAPI interface {
	Datacenters() ([]string, error)
	Service(service, tag string, q *api.QueryOptions) ([]*api.CatalogService, *api.QueryMeta, error)
}

// NamespaceAPI is the consul/api.Namespace API used by Nomad.
//
// ACL requirements
// - operator:read OR namespace:*:read
type NamespaceAPI interface {
	List(q *api.QueryOptions) ([]*api.Namespace, *api.QueryMeta, error)
}

// AgentAPI is the consul/api.Agent API used by Nomad.
//
// ACL requirements
// - agent:read
// - service:write
type AgentAPI interface {
	CheckRegister(check *api.AgentCheckRegistration) error
	CheckDeregisterOpts(checkID string, q *api.QueryOptions) error
	ChecksWithFilterOpts(filter string, q *api.QueryOptions) (map[string]*api.AgentCheck, error)
	UpdateTTLOpts(id, output, status string, q *api.QueryOptions) error

	ServiceRegister(service *api.AgentServiceRegistration) error
	ServiceDeregisterOpts(serviceID string, q *api.QueryOptions) error
	ServicesWithFilterOpts(filter string, q *api.QueryOptions) (map[string]*api.AgentService, error)

	Self() (map[string]map[string]interface{}, error)
}

// ConfigAPI is the consul/api.ConfigEntries API subset used by Nomad Server.
//
// ACL requirements
// - operator:write (server only)
type ConfigAPI interface {
	Set(entry api.ConfigEntry, w *api.WriteOptions) (bool, *api.WriteMeta, error)
	// Delete(kind, name string, w *api.WriteOptions) (*api.WriteMeta, error) (not used)
}

// ACLsAPI is the consul/api.ACL API subset used by Nomad Server.
//
// ACL requirements
// - acl:write (server only)
type ACLsAPI interface {
	TokenReadSelf(q *api.QueryOptions) (*api.ACLToken, *api.QueryMeta, error) // for lookup via operator token
	PolicyRead(policyID string, q *api.QueryOptions) (*api.ACLPolicy, *api.QueryMeta, error)
	RoleRead(roleID string, q *api.QueryOptions) (*api.ACLRole, *api.QueryMeta, error)
	TokenCreate(partial *api.ACLToken, q *api.WriteOptions) (*api.ACLToken, *api.WriteMeta, error)
	TokenDelete(accessorID string, q *api.WriteOptions) (*api.WriteMeta, error)
	TokenList(q *api.QueryOptions) ([]*api.ACLTokenListEntry, *api.QueryMeta, error)
}

// agentServiceUpdateRequired checks if any critical fields in Nomad's version
// of a service definition are different from the existing service definition as
// known by Consul.
//
//	reason - The syncReason that triggered this synchronization with the consul
//	         agent API.
//	wanted - Nomad's view of what the service definition is intended to be.
//	         Not nil.
//	existing - Consul's view (agent, not catalog) of the actual service definition.
//	         Not nil.
//	sidecar - Consul's view (agent, not catalog) of the service definition of the sidecar
//	         associated with existing that may or may not exist.
//	         May be nil.
func (s *ServiceClient) agentServiceUpdateRequired(reason syncReason, wanted *api.AgentServiceRegistration, existing *api.AgentService, sidecar *api.AgentService) bool {
	switch reason {
	case syncPeriodic:
		// In a periodic sync with Consul, we need to respect the value of
		// the enable_tag_override field so that we maintain the illusion that the
		// user is in control of the Consul tags, as they may be externally edited
		// via the Consul catalog API (e.g. a user manually sets them).
		//
		// As Consul does by disabling anti-entropy for the tags field, Nomad will
		// ignore differences in the tags field during the periodic syncs with
		// the Consul agent API.
		//
		// We do so by over-writing the nomad service registration by the value
		// of the tags that Consul contains, if enable_tag_override = true.
		maybeTweakTags(wanted, existing, sidecar)

		// Also, purge tagged address fields of nomad agent services.
		maybeTweakTaggedAddresses(wanted, existing)

		// Okay now it is safe to compare.
		return s.different(wanted, existing, sidecar)

	default:
		// A non-periodic sync with Consul indicates an operation has been set
		// on the queue. This happens when service has been added / removed / modified
		// and implies the Consul agent should be sync'd with nomad, because
		// nomad is the ultimate source of truth for the service definition.

		// But do purge tagged address fields of nomad agent services.
		maybeTweakTaggedAddresses(wanted, existing)

		// Okay now it is safe to compare.
		return s.different(wanted, existing, sidecar)
	}
}

// maybeTweakTags will override wanted.Tags with a copy of existing.Tags only if
// EnableTagOverride is true. Otherwise the wanted service registration is left
// unchanged.
func maybeTweakTags(wanted *api.AgentServiceRegistration, existing *api.AgentService, sidecar *api.AgentService) {
	if wanted.EnableTagOverride {
		wanted.Tags = slices.Clone(existing.Tags)
		// If the service registration also defines a sidecar service, use the ETO
		// setting for the parent service to also apply to the sidecar.
		if wanted.Connect != nil && wanted.Connect.SidecarService != nil {
			if sidecar != nil {
				wanted.Connect.SidecarService.Tags = slices.Clone(sidecar.Tags)
			}
		}
	}
}

// maybeTweakTaggedAddresses will remove the Consul-injected .TaggedAddresses fields
// from existing if wanted represents a Nomad agent (Client or Server) or Nomad managed
// service, which do not themselves configure those tagged addresses. We do this
// because Consul will magically set the .TaggedAddress to values Nomad does not
// know about if they are submitted as unset.
func maybeTweakTaggedAddresses(wanted *api.AgentServiceRegistration, existing *api.AgentService) {
	if isNomadAgent(wanted.ID) || isNomadService(wanted.ID) {
		if _, exists := wanted.TaggedAddresses["lan_ipv4"]; !exists {
			delete(existing.TaggedAddresses, "lan_ipv4")
		}
		if _, exists := wanted.TaggedAddresses["wan_ipv4"]; !exists {
			delete(existing.TaggedAddresses, "wan_ipv4")
		}
		if _, exists := wanted.TaggedAddresses["lan_ipv6"]; !exists {
			delete(existing.TaggedAddresses, "lan_ipv6")
		}
		if _, exists := wanted.TaggedAddresses["wan_ipv6"]; !exists {
			delete(existing.TaggedAddresses, "wan_ipv6")
		}
	}
}

// different compares the wanted state of the service registration with the actual
// (cached) state of the service registration reported by Consul. If any of the
// critical fields are not deeply equal, they considered different.
func (s *ServiceClient) different(wanted *api.AgentServiceRegistration, existing *api.AgentService, sidecar *api.AgentService) bool {
	trace := func(field string, left, right any) {
		s.logger.Trace("registrations different", "id", wanted.ID,
			"field", field, "wanted", fmt.Sprintf("%#v", left), "existing", fmt.Sprintf("%#v", right),
		)
	}

	switch {
	case wanted.Kind != existing.Kind:
		trace("kind", wanted.Kind, existing.Kind)
		return true
	case wanted.ID != existing.ID:
		trace("id", wanted.ID, existing.ID)
		return true
	case wanted.Port != existing.Port:
		trace("port", wanted.Port, existing.Port)
		return true
	case wanted.Address != existing.Address:
		trace("address", wanted.Address, existing.Address)
		return true
	case wanted.Name != existing.Service:
		trace("service name", wanted.Name, existing.Service)
		return true
	case wanted.EnableTagOverride != existing.EnableTagOverride:
		trace("enable_tag_override", wanted.EnableTagOverride, existing.EnableTagOverride)
		return true
	case !maps.Equal(wanted.Meta, existing.Meta):
		trace("meta", wanted.Meta, existing.Meta)
		return true
	case !maps.Equal(wanted.TaggedAddresses, existing.TaggedAddresses):
		trace("tagged_addresses", wanted.TaggedAddresses, existing.TaggedAddresses)
		return true
	case !helper.SliceSetEq(wanted.Tags, existing.Tags):
		trace("tags", wanted.Tags, existing.Tags)
		return true
	case connectSidecarDifferent(wanted, sidecar):
		trace("connect_sidecar", wanted.Name, existing.Service)
		return true
	}
	return false
}

// sidecarTagsDifferent includes the special logic for comparing sidecar tags
// from Nomad vs. Consul perspective. Because Consul forces the sidecar tags
// to inherit the parent service tags if the sidecar tags are unset, we need to
// take that into consideration when Nomad's sidecar tags are unset by instead
// comparing them to the parent service tags.
func sidecarTagsDifferent(parent, wanted, sidecar []string) bool {
	if len(wanted) == 0 {
		return !helper.SliceSetEq(parent, sidecar)
	}
	return !helper.SliceSetEq(wanted, sidecar)
}

// proxyUpstreamsDifferent determines if the sidecar_service.proxy.upstreams
// configurations are different between the desired sidecar service state, and
// the actual sidecar service state currently registered in Consul.
func proxyUpstreamsDifferent(wanted *api.AgentServiceConnect, sidecar *api.AgentServiceConnectProxyConfig) bool {
	// There is similar code that already does this in Nomad's API package,
	// however here we are operating on Consul API package structs, and they do not
	// provide such helper functions.

	getProxyUpstreams := func(pc *api.AgentServiceConnectProxyConfig) []api.Upstream {
		switch {
		case pc == nil:
			return nil
		case len(pc.Upstreams) == 0:
			return nil
		default:
			return pc.Upstreams
		}
	}

	getConnectUpstreams := func(sc *api.AgentServiceConnect) []api.Upstream {
		switch {
		case sc.SidecarService.Proxy == nil:
			return nil
		case len(sc.SidecarService.Proxy.Upstreams) == 0:
			return nil
		default:
			return sc.SidecarService.Proxy.Upstreams
		}
	}

	upstreamsDifferent := func(a, b []api.Upstream) bool {
		if len(a) != len(b) {
			return true
		}

		for i := 0; i < len(a); i++ {
			A := a[i]
			B := b[i]
			switch {
			case A.Datacenter != B.Datacenter:
				return true
			case A.DestinationName != B.DestinationName:
				return true
			case A.LocalBindAddress != B.LocalBindAddress:
				return true
			case A.LocalBindPort != B.LocalBindPort:
				return true
			case A.MeshGateway.Mode != B.MeshGateway.Mode:
				return true
			case !reflect.DeepEqual(A.Config, B.Config):
				return true
			}
		}
		return false
	}

	return upstreamsDifferent(
		getConnectUpstreams(wanted),
		getProxyUpstreams(sidecar),
	)
}

// connectSidecarDifferent returns true if Nomad expects there to be a sidecar
// hanging off the desired parent service definition on the Consul side, and does
// not match with what Consul has.
//
// This is used to determine if the connect sidecar service registration should be
// updated - potentially (but not necessarily) in-place.
func connectSidecarDifferent(wanted *api.AgentServiceRegistration, sidecar *api.AgentService) bool {
	if wanted.Connect != nil && wanted.Connect.SidecarService != nil {
		if sidecar == nil {
			// consul lost our sidecar (?)
			return true
		}

		if sidecarTagsDifferent(wanted.Tags, wanted.Connect.SidecarService.Tags, sidecar.Tags) {
			// tags on the nomad definition have been modified
			return true
		}

		if proxyUpstreamsDifferent(wanted.Connect, sidecar.Proxy) {
			// proxy upstreams on the nomad definition have been modified
			return true
		}
	}

	// Either Nomad does not expect there to be a sidecar_service, or there is
	// no actionable difference from the Consul sidecar_service definition.
	return false
}

// operations are submitted to the main loop via commit() for synchronizing
// with Consul.
type operations struct {
	regServices   []*api.AgentServiceRegistration
	regChecks     []*api.AgentCheckRegistration
	deregServices []string
	deregChecks   []string
}

func (o *operations) empty() bool {
	switch {
	case o == nil:
		return true
	case len(o.regServices) > 0:
		return false
	case len(o.regChecks) > 0:
		return false
	case len(o.deregServices) > 0:
		return false
	case len(o.deregChecks) > 0:
		return false
	default:
		return true
	}
}

func (o *operations) String() string {
	return fmt.Sprintf("<%d, %d, %d, %d>", len(o.regServices), len(o.regChecks), len(o.deregServices), len(o.deregChecks))
}

// ServiceClient handles task and agent service registration with Consul.
type ServiceClient struct {
	agentAPI         AgentAPI
	namespacesClient *NamespacesClient

	logger           hclog.Logger
	retryInterval    time.Duration
	maxRetryInterval time.Duration
	periodicInterval time.Duration

	// exitCh is closed when the main Run loop exits
	exitCh chan struct{}

	// shutdownCh is closed when the client should shutdown
	shutdownCh chan struct{}

	// shutdownWait is how long Shutdown() blocks waiting for the final
	// sync() to finish. Defaults to defaultShutdownWait
	shutdownWait time.Duration

	opCh chan *operations

	services map[string]*api.AgentServiceRegistration
	checks   map[string]*api.AgentCheckRegistration

	explicitlyDeregisteredServices *set.Set[string]
	explicitlyDeregisteredChecks   *set.Set[string]

	// allocRegistrations stores the services and checks that are registered
	// with Consul by allocation ID.
	allocRegistrations     map[string]*serviceregistration.AllocRegistration
	allocRegistrationsLock sync.RWMutex

	// Nomad agent services and checks that are recorded so they can be removed
	// on shutdown. Defers to consul namespace specified in client consul config.
	agentServices *set.Set[string]
	agentChecks   *set.Set[string]
	agentLock     sync.Mutex

	// seen is 1 if Consul has ever been seen; otherwise 0. Accessed with
	// atomics.
	seen int32

	// deregisterProbationExpiry is the time before which consul sync shouldn't deregister
	// unknown services.
	// Used to mitigate risk of deleting restored services upon client restart.
	deregisterProbationExpiry time.Time

	// checkWatcher restarts checks that are unhealthy.
	checkWatcher *serviceregistration.UniversalCheckWatcher

	// isClientAgent specifies whether this Consul client is being used
	// by a Nomad client.
	isClientAgent bool
}

// checkStatusGetter is the consul-specific implementation of serviceregistration.CheckStatusGetter
type checkStatusGetter struct {
	agentAPI         AgentAPI
	namespacesClient *NamespacesClient
}

func (csg *checkStatusGetter) Get() (map[string]string, error) {
	// Get the list of all namespaces so we can iterate them.
	namespaces, err := csg.namespacesClient.List()
	if err != nil {
		return nil, err
	}

	results := make(map[string]string)
	for _, namespace := range namespaces {
		resultsInNamespace, err := csg.agentAPI.ChecksWithFilterOpts("", &api.QueryOptions{Namespace: normalizeNamespace(namespace)})
		if err != nil {
			return nil, err
		}

		for k, v := range resultsInNamespace {
			results[k] = v.Status
		}
	}
	return results, nil
}

// NewServiceClient creates a new Consul ServiceClient from an existing Consul API
// Client, logger and takes whether the client is being used by a Nomad Client agent.
// When being used by a Nomad client, this Consul client reconciles all services and
// checks created by Nomad on behalf of running tasks.
func NewServiceClient(agentAPI AgentAPI, namespacesClient *NamespacesClient, logger hclog.Logger, isNomadClient bool) *ServiceClient {
	logger = logger.ResetNamed("consul.sync")
	return &ServiceClient{
		agentAPI:                       agentAPI,
		namespacesClient:               namespacesClient,
		logger:                         logger,
		retryInterval:                  defaultRetryInterval,
		maxRetryInterval:               defaultMaxRetryInterval,
		periodicInterval:               defaultPeriodicInterval,
		exitCh:                         make(chan struct{}),
		shutdownCh:                     make(chan struct{}),
		shutdownWait:                   defaultShutdownWait,
		opCh:                           make(chan *operations, 8),
		services:                       make(map[string]*api.AgentServiceRegistration),
		checks:                         make(map[string]*api.AgentCheckRegistration),
		explicitlyDeregisteredServices: set.New[string](0),
		explicitlyDeregisteredChecks:   set.New[string](0),
		allocRegistrations:             make(map[string]*serviceregistration.AllocRegistration),
		agentServices:                  set.New[string](4),
		agentChecks:                    set.New[string](0),
		isClientAgent:                  isNomadClient,
		deregisterProbationExpiry:      time.Now().Add(deregisterProbationPeriod),
		checkWatcher: serviceregistration.NewCheckWatcher(logger, &checkStatusGetter{
			agentAPI:         agentAPI,
			namespacesClient: namespacesClient,
		}),
	}
}

// seen is used by markSeen and hasSeen
const seen = 1

// markSeen marks Consul as having been seen (meaning at least one operation
// has succeeded).
func (c *ServiceClient) markSeen() {
	atomic.StoreInt32(&c.seen, seen)
}

// hasSeen returns true if any Consul operation has ever succeeded. Useful to
// squelch errors if Consul isn't running.
func (c *ServiceClient) hasSeen() bool {
	return atomic.LoadInt32(&c.seen) == seen
}

// syncReason indicates why a sync operation with consul is about to happen.
//
// The trigger for a sync may have implications on the behavior of the sync itself.
// In particular if a service is defined with enable_tag_override=true, the sync
// should ignore changes to the service's Tags field.
type syncReason byte

const (
	syncPeriodic syncReason = iota
	syncShutdown
	syncNewOps
)

func (sr syncReason) String() string {
	switch sr {
	case syncPeriodic:
		return "periodic"
	case syncShutdown:
		return "shutdown"
	case syncNewOps:
		return "operations"
	default:
		return "unexpected"
	}
}

// Run the Consul main loop which retries operations against Consul. It should
// be called exactly once.
func (c *ServiceClient) Run() {
	defer close(c.exitCh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init will be closed when Consul has been contacted
	init := make(chan struct{})
	go checkConsulTLSSkipVerify(ctx, c.logger, c.agentAPI, init)

	// Process operations while waiting for initial contact with Consul but
	// do not sync until contact has been made.
INIT:
	for {
		select {
		case <-init:
			c.markSeen()
			break INIT
		case <-c.shutdownCh:
			return
		case ops := <-c.opCh:
			c.merge(ops)
		}
	}
	c.logger.Trace("able to contact Consul")

	// Block until contact with Consul has been established
	// Start checkWatcher
	go c.checkWatcher.Run(ctx)

	// Always immediately sync to reconcile Nomad and Consul's state
	retryTimer := time.NewTimer(0)

	failures := 0
	for {
		// On every iteration take note of what the trigger for the next sync
		// was, so that it may be referenced during the sync itself.
		var reasonForSync syncReason

		select {
		case <-retryTimer.C:
			reasonForSync = syncPeriodic
		case <-c.shutdownCh:
			reasonForSync = syncShutdown
			// Cancel check watcher but sync one last time
			cancel()
		case ops := <-c.opCh:
			reasonForSync = syncNewOps
			c.merge(ops)
		}

		if err := c.sync(reasonForSync); err != nil {
			if failures == 0 {
				// Log on the first failure
				c.logger.Warn("failed to update services in Consul", "error", err)
			} else if failures%10 == 0 {
				// Log every 10th consecutive failure
				c.logger.Error("still unable to update services in Consul", "failures", failures, "error", err)
			}

			failures++
			if !retryTimer.Stop() {
				// Timer already expired, since the timer may
				// or may not have been read in the select{}
				// above, conditionally receive on it
				select {
				case <-retryTimer.C:
				default:
				}
			}
			backoff := c.retryInterval * time.Duration(failures)
			if backoff > c.maxRetryInterval {
				backoff = c.maxRetryInterval
			}
			retryTimer.Reset(backoff)
		} else {
			if failures > 0 {
				c.logger.Info("successfully updated services in Consul")
				failures = 0
			}

			// on successful sync, clear deregistered consul entities
			c.clearExplicitlyDeregistered()

			// Reset timer to periodic interval to periodically
			// reconile with Consul
			if !retryTimer.Stop() {
				select {
				case <-retryTimer.C:
				default:
				}
			}
			retryTimer.Reset(c.periodicInterval)
		}

		select {
		case <-c.shutdownCh:
			// Exit only after sync'ing all outstanding operations
			if len(c.opCh) > 0 {
				for len(c.opCh) > 0 {
					c.merge(<-c.opCh)
				}
				continue
			}
			return
		default:
		}

	}
}

// commit operations unless already shutting down.
func (c *ServiceClient) commit(ops *operations) {
	c.logger.Trace("commit sync operations", "ops", ops)

	// Ignore empty operations - ideally callers will optimize out syncs with
	// nothing to do, but be defensive anyway. Sending an empty ops on the chan
	// will trigger an unnecessary sync with Consul.
	if ops.empty() {
		return
	}

	// Prioritize doing nothing if we are being signaled to shutdown.
	select {
	case <-c.shutdownCh:
		return
	default:
	}

	// Send the ops down the ops chan, triggering a sync with Consul. Unless we
	// receive a signal to shutdown.
	select {
	case c.opCh <- ops:
	case <-c.shutdownCh:
	}
}

func (c *ServiceClient) clearExplicitlyDeregistered() {
	c.explicitlyDeregisteredServices = set.New[string](0)
	c.explicitlyDeregisteredChecks = set.New[string](0)
}

// merge registrations into state map prior to sync'ing with Consul
func (c *ServiceClient) merge(ops *operations) {
	for _, s := range ops.regServices {
		c.services[s.ID] = s
	}
	for _, check := range ops.regChecks {
		c.checks[check.ID] = check
	}
	for _, sid := range ops.deregServices {
		delete(c.services, sid)
		c.explicitlyDeregisteredServices.Insert(sid)
	}
	for _, cid := range ops.deregChecks {
		delete(c.checks, cid)
		c.explicitlyDeregisteredChecks.Insert(cid)
	}
	metrics.SetGauge([]string{"client", "consul", "services"}, float32(len(c.services)))
	metrics.SetGauge([]string{"client", "consul", "checks"}, float32(len(c.checks)))
}

// sync enqueued operations.
func (c *ServiceClient) sync(reason syncReason) error {
	c.logger.Trace("execute sync", "reason", reason)

	sreg, creg, sdereg, cdereg := 0, 0, 0, 0
	var err error

	// Get the list of all namespaces created so we can iterate them.
	namespaces, err := c.namespacesClient.List()
	if err != nil {
		metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
		return fmt.Errorf("failed to query Consul namespaces: %w", err)
	}

	// Accumulate all services in Consul across all namespaces.
	servicesInConsul := make(map[string]*api.AgentService)
	for _, namespace := range namespaces {
		if nsServices, err := c.agentAPI.ServicesWithFilterOpts("", &api.QueryOptions{Namespace: normalizeNamespace(namespace)}); err != nil {
			metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
			return fmt.Errorf("failed to query Consul services: %w", err)
		} else {
			for k, v := range nsServices {
				servicesInConsul[k] = v
			}
		}
	}

	// Compute whether we are still in probation period where we will avoid
	// de-registering services.
	inProbation := time.Now().Before(c.deregisterProbationExpiry)

	// Remove Nomad services in Consul but unknown to Nomad.
	for id := range servicesInConsul {
		if _, ok := c.services[id]; ok {
			// Known service, skip
			continue
		}

		// Ignore if this is not a Nomad managed service. Also ignore
		// Nomad managed services if this is not a client agent.
		// This is to prevent server agents from removing services
		// registered by client agents
		if !isNomadService(id) || !c.isClientAgent {
			// Not managed by Nomad, skip
			continue
		}

		// Ignore unknown services during probation
		if inProbation && !c.explicitlyDeregisteredServices.Contains(id) {
			continue
		}

		// Ignore if this is a service for a Nomad managed sidecar proxy.
		if maybeConnectSidecar(id) {
			continue
		}

		// Get the Consul namespace this service is in.
		ns := servicesInConsul[id].Namespace

		// If this service has a sidecar, we need to remove the sidecar first,
		// otherwise Consul will produce a warning and an error when removing
		// the parent service.
		//
		// The sidecar is not tracked on the Nomad side; it was registered
		// implicitly through the parent service.
		if sidecar := getNomadSidecar(id, servicesInConsul); sidecar != nil {
			if err := c.agentAPI.ServiceDeregisterOpts(sidecar.ID, &api.QueryOptions{Namespace: ns}); err != nil {
				metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
				return err
			}
		}

		// Remove the unwanted service.
		if err := c.agentAPI.ServiceDeregisterOpts(id, &api.QueryOptions{Namespace: ns}); err != nil {
			if isOldNomadService(id) {
				// Don't hard-fail on old entries. See #3620
				continue
			}

			metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
			return err
		}
		sdereg++
		metrics.IncrCounter([]string{"client", "consul", "service_deregistrations"}, 1)
	}

	// Add Nomad managed services missing in Consul, or updated via Nomad.
	for id, serviceInNomad := range c.services {
		serviceInConsul, exists := servicesInConsul[id]
		sidecarInConsul := getNomadSidecar(id, servicesInConsul)

		if !exists || c.agentServiceUpdateRequired(reason, serviceInNomad, serviceInConsul, sidecarInConsul) {
			c.logger.Trace("must register service", "id", id, "exists", exists, "reason", reason)
			if err = c.agentAPI.ServiceRegister(serviceInNomad); err != nil {
				metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
				return err
			}
			sreg++
			metrics.IncrCounter([]string{"client", "consul", "service_registrations"}, 1)
		}

	}

	checksInConsul := make(map[string]*api.AgentCheck)
	for _, namespace := range namespaces {
		nsChecks, err := c.agentAPI.ChecksWithFilterOpts("", &api.QueryOptions{Namespace: normalizeNamespace(namespace)})
		if err != nil {
			metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
			return fmt.Errorf("failed to query Consul checks: %w", err)
		}
		for k, v := range nsChecks {
			checksInConsul[k] = v
		}
	}

	// Remove Nomad checks in Consul but unknown locally
	for id, check := range checksInConsul {
		if _, ok := c.checks[id]; ok {
			// Known check, leave it
			continue
		}

		// Ignore if this is not a Nomad managed check. Also ignore
		// Nomad managed checks if this is not a client agent.
		// This is to prevent server agents from removing checks
		// registered by client agents
		if !isNomadService(check.ServiceID) || !c.isClientAgent || !isNomadCheck(check.CheckID) {
			// Service not managed by Nomad, skip
			continue
		}

		// Ignore unknown services during probation
		if inProbation && !c.explicitlyDeregisteredChecks.Contains(id) {
			continue
		}

		// Ignore if this is a check for a Nomad managed sidecar proxy.
		if maybeSidecarProxyCheck(id) {
			continue
		}

		// Unknown Nomad managed check; remove
		if err := c.agentAPI.CheckDeregisterOpts(id, &api.QueryOptions{Namespace: check.Namespace}); err != nil {
			if isOldNomadService(check.ServiceID) {
				// Don't hard-fail on old entries.
				continue
			}

			metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
			return err
		}
		cdereg++
		metrics.IncrCounter([]string{"client", "consul", "check_deregistrations"}, 1)
	}

	// Add Nomad checks missing from Consul
	for id, check := range c.checks {
		if _, ok := checksInConsul[id]; ok {
			// Already in Consul; skipping
			continue
		}
		if err := c.agentAPI.CheckRegister(check); err != nil {
			metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
			return err
		}
		creg++
		metrics.IncrCounter([]string{"client", "consul", "check_registrations"}, 1)
	}

	// Only log if something was actually synced
	if sreg > 0 || sdereg > 0 || creg > 0 || cdereg > 0 {
		c.logger.Debug("sync complete", "registered_services", sreg, "deregistered_services", sdereg,
			"registered_checks", creg, "deregistered_checks", cdereg)
	}
	return nil
}

// RegisterAgent registers Nomad agents (client or server). The
// Service.PortLabel should be a literal port to be parsed with SplitHostPort.
// Script checks are not supported and will return an error. Registration is
// asynchronous.
//
// Agents will be deregistered when Shutdown is called.
//
// Note: no need to manually plumb Consul namespace into the agent service registration
// or its check registrations, because the Nomad Client's Consul Client will already
// have the Nomad Client's Consul Namespace set on startup.
func (c *ServiceClient) RegisterAgent(role string, services []*structs.Service) error {
	ops := operations{}

	for _, service := range services {
		id := makeAgentServiceID(role, service)

		// Unlike tasks, agents don't use port labels. Agent ports are
		// stored directly in the PortLabel.
		host, rawport, err := net.SplitHostPort(service.PortLabel)
		if err != nil {
			return fmt.Errorf("error parsing port label %q from service %q: %v", service.PortLabel, service.Name, err)
		}
		port, err := strconv.Atoi(rawport)
		if err != nil {
			return fmt.Errorf("error parsing port %q from service %q: %v", rawport, service.Name, err)
		}
		serviceReg := &api.AgentServiceRegistration{
			ID:      id,
			Name:    service.Name,
			Tags:    service.Tags,
			Address: host,
			Port:    port,
			// This enables the consul UI to show that Nomad registered this service
			Meta: map[string]string{
				"external-source": "nomad",
			},
		}
		ops.regServices = append(ops.regServices, serviceReg)

		for _, check := range service.Checks {
			checkID := MakeCheckID(id, check)
			if check.Type == structs.ServiceCheckScript {
				return fmt.Errorf("service %q contains invalid check: agent checks do not support scripts", service.Name)
			}
			checkHost, checkPort := serviceReg.Address, serviceReg.Port
			if check.PortLabel != "" {
				// Unlike tasks, agents don't use port labels. Agent ports are
				// stored directly in the PortLabel.
				host, rawport, err := net.SplitHostPort(check.PortLabel)
				if err != nil {
					return fmt.Errorf("error parsing port label %q from check %q: %v", service.PortLabel, check.Name, err)
				}
				port, err := strconv.Atoi(rawport)
				if err != nil {
					return fmt.Errorf("error parsing port %q from check %q: %v", rawport, check.Name, err)
				}
				checkHost, checkPort = host, port
			}
			checkReg, err := createCheckReg(id, checkID, check, checkHost, checkPort, "")
			if err != nil {
				return fmt.Errorf("failed to add check %q: %v", check.Name, err)
			}
			ops.regChecks = append(ops.regChecks, checkReg)
		}
	}

	// Don't bother committing agent checks if we're already shutting down
	c.agentLock.Lock()
	defer c.agentLock.Unlock()
	select {
	case <-c.shutdownCh:
		return nil
	default:
	}

	// Now add them to the registration queue
	c.commit(&ops)

	// Record IDs for deregistering on shutdown
	for _, id := range ops.regServices {
		c.agentServices.Insert(id.ID)
	}
	for _, id := range ops.regChecks {
		c.agentChecks.Insert(id.ID)
	}
	return nil
}

// serviceRegs creates service registrations, check registrations, and script
// checks from a service. It returns a service registration object with the
// service and check IDs populated.
func (c *ServiceClient) serviceRegs(
	ops *operations,
	service *structs.Service,
	workload *serviceregistration.WorkloadServices,
) (*serviceregistration.ServiceRegistration, error) {

	// Get the services ID
	id := serviceregistration.MakeAllocServiceID(workload.AllocInfo.AllocID, workload.Name(), service)
	sreg := &serviceregistration.ServiceRegistration{
		ServiceID:     id,
		CheckIDs:      make(map[string]struct{}, len(service.Checks)),
		CheckOnUpdate: make(map[string]string, len(service.Checks)),
	}

	// Service address modes default to auto
	addrMode := service.AddressMode
	if addrMode == "" {
		addrMode = structs.AddressModeAuto
	}

	// Determine the address to advertise based on the mode
	ip, port, err := serviceregistration.GetAddress(
		service.Address, addrMode, service.PortLabel, workload.Networks, workload.DriverNetwork, workload.Ports, workload.NetworkStatus)
	if err != nil {
		return nil, fmt.Errorf("unable to get address for service %q: %v", service.Name, err)
	}

	// Determine whether to use tags or canary_tags
	var tags []string
	if workload.Canary && len(service.CanaryTags) > 0 {
		tags = make([]string, len(service.CanaryTags))
		copy(tags, service.CanaryTags)
	} else {
		tags = make([]string, len(service.Tags))
		copy(tags, service.Tags)
	}

	// newConnect returns (nil, nil) if there's no Connect-enabled service.
	connect, err := newConnect(id, workload.AllocInfo, service.Name, service.Connect, workload.Networks, workload.Ports)
	if err != nil {
		return nil, fmt.Errorf("invalid Consul Connect configuration for service %q: %v", service.Name, err)
	}

	// newConnectGateway returns nil if there's no Connect gateway.
	gateway := newConnectGateway(service.Connect)

	// Determine whether to use meta or canary_meta
	var meta map[string]string
	if workload.Canary && len(service.CanaryMeta) > 0 {
		meta = make(map[string]string, len(service.CanaryMeta)+1)
		for k, v := range service.CanaryMeta {
			meta[k] = v
		}
	} else {
		meta = make(map[string]string, len(service.Meta)+1)
		for k, v := range service.Meta {
			meta[k] = v
		}
	}

	// This enables the consul UI to show that Nomad registered this service
	meta["external-source"] = "nomad"

	// Explicitly set the Consul service Kind in case this service represents
	// one of the Connect gateway types.
	kind := api.ServiceKindTypical
	switch {
	case service.Connect.IsIngress():
		kind = api.ServiceKindIngressGateway
	case service.Connect.IsTerminating():
		kind = api.ServiceKindTerminatingGateway

		if proxy := service.Connect.Gateway.Proxy; proxy != nil {
			// set the default port if bridge / default listener set
			if defaultBind, exists := proxy.EnvoyGatewayBindAddresses["default"]; exists {
				portLabel := envoy.PortLabel(structs.ConnectTerminatingPrefix, service.Name, "")
				if dynPort, ok := workload.Ports.Get(portLabel); ok {
					defaultBind.Port = dynPort.Value
				}
			}
		}
	case service.Connect.IsMesh():
		kind = api.ServiceKindMeshGateway

		if proxy := service.Connect.Gateway.Proxy; proxy != nil {
			// wan uses the service port label, which is typically on a discrete host_network
			if wanBind, exists := proxy.EnvoyGatewayBindAddresses["wan"]; exists {
				if wanPort, ok := workload.Ports.Get(service.PortLabel); ok {
					wanBind.Port = wanPort.Value
				}
			}
			// lan uses a nomad generated dynamic port on the default network
			if lanBind, exists := proxy.EnvoyGatewayBindAddresses["lan"]; exists {
				portLabel := envoy.PortLabel(structs.ConnectMeshPrefix, service.Name, "lan")
				if dynPort, ok := workload.Ports.Get(portLabel); ok {
					lanBind.Port = dynPort.Value
				}
			}
		}
	}

	taggedAddresses, err := parseTaggedAddresses(service.TaggedAddresses, port)
	if err != nil {
		return nil, err
	}

	// Build the Consul Service registration request
	serviceReg := &api.AgentServiceRegistration{
		Kind:              kind,
		ID:                id,
		Name:              service.Name,
		Namespace:         workload.ProviderNamespace,
		Tags:              tags,
		EnableTagOverride: service.EnableTagOverride,
		Address:           ip,
		Port:              port,
		Meta:              meta,
		TaggedAddresses:   taggedAddresses,
		Connect:           connect, // will be nil if no Connect block
		Proxy:             gateway, // will be nil if no Connect Gateway block
		Checks:            make([]*api.AgentServiceCheck, 0, len(service.Checks)),
	}
	ops.regServices = append(ops.regServices, serviceReg)

	// Build the check registrations
	checkRegs, err := c.checkRegs(id, service, workload, sreg)
	if err != nil {
		return nil, err
	}

	for _, registration := range checkRegs {
		sreg.CheckIDs[registration.ID] = struct{}{}
		ops.regChecks = append(ops.regChecks, registration)
		serviceReg.Checks = append(
			serviceReg.Checks,
			apiCheckRegistrationToCheck(registration),
		)
	}

	return sreg, nil
}

// apiCheckRegistrationToCheck converts a check registration to a check, so that
// we can include them in the initial service registration. It is expected the
// Nomad-conversion (e.g. turning script checks into ttl checks) has already been
// applied.
func apiCheckRegistrationToCheck(r *api.AgentCheckRegistration) *api.AgentServiceCheck {
	return &api.AgentServiceCheck{
		CheckID:                r.ID,
		Name:                   r.Name,
		Interval:               r.Interval,
		Timeout:                r.Timeout,
		TTL:                    r.TTL,
		HTTP:                   r.HTTP,
		Header:                 maps.Clone(r.Header),
		Method:                 r.Method,
		Body:                   r.Body,
		TCP:                    r.TCP,
		Status:                 r.Status,
		TLSServerName:          r.TLSServerName,
		TLSSkipVerify:          r.TLSSkipVerify,
		GRPC:                   r.GRPC,
		GRPCUseTLS:             r.GRPCUseTLS,
		SuccessBeforePassing:   r.SuccessBeforePassing,
		FailuresBeforeCritical: r.FailuresBeforeCritical,
	}
}

// checkRegs creates check registrations for the given service
func (c *ServiceClient) checkRegs(
	serviceID string,
	service *structs.Service,
	workload *serviceregistration.WorkloadServices,
	sreg *serviceregistration.ServiceRegistration,
) ([]*api.AgentCheckRegistration, error) {

	registrations := make([]*api.AgentCheckRegistration, 0, len(service.Checks))
	for _, check := range service.Checks {
		var ip string
		var port int

		if check.Type != structs.ServiceCheckScript {
			portLabel := check.PortLabel
			if portLabel == "" {
				portLabel = service.PortLabel
			}

			addrMode := check.AddressMode
			if addrMode == "" {
				if service.Address != "" {
					// if the service is using a custom address, enable the check
					// to use that address
					addrMode = structs.AddressModeAuto
				} else {
					// otherwise default to the host address
					addrMode = structs.AddressModeHost
				}
			}

			var err error
			ip, port, err = serviceregistration.GetAddress(
				service.Address, addrMode, portLabel, workload.Networks, workload.DriverNetwork, workload.Ports, workload.NetworkStatus)
			if err != nil {
				return nil, fmt.Errorf("error getting address for check %q: %v", check.Name, err)
			}
		}

		checkID := MakeCheckID(serviceID, check)
		registration, err := createCheckReg(serviceID, checkID, check, ip, port, workload.ProviderNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to add check %q: %v", check.Name, err)
		}
		sreg.CheckOnUpdate[checkID] = check.OnUpdate
		registrations = append(registrations, registration)
	}

	return registrations, nil
}

// RegisterWorkload with Consul. Adds all service entries and checks to Consul.
//
// If the service IP is set it used as the address in the service registration.
// Checks will always use the IP from the Task struct (host's IP).
//
// Actual communication with Consul is done asynchronously (see Run).
func (c *ServiceClient) RegisterWorkload(workload *serviceregistration.WorkloadServices) error {
	// Fast path
	numServices := len(workload.Services)
	if numServices == 0 {
		return nil
	}

	t := new(serviceregistration.ServiceRegistrations)
	t.Services = make(map[string]*serviceregistration.ServiceRegistration, numServices)

	ops := &operations{}
	for _, service := range workload.Services {
		sreg, err := c.serviceRegs(ops, service, workload)
		if err != nil {
			return err
		}
		t.Services[sreg.ServiceID] = sreg
	}

	// Add the workload to the allocation's registration
	c.addRegistrations(workload.AllocInfo.AllocID, workload.Name(), t)

	c.commit(ops)

	// Start watching checks. Done after service registrations are built
	// since an error building them could leak watches.
	for _, service := range workload.Services {
		serviceID := serviceregistration.MakeAllocServiceID(workload.AllocInfo.AllocID, workload.Name(), service)
		for _, check := range service.Checks {
			if check.TriggersRestarts() {
				checkID := MakeCheckID(serviceID, check)
				c.checkWatcher.Watch(workload.AllocInfo.AllocID, workload.Name(), checkID, check, workload.Restarter)
			}
		}
	}
	return nil
}

// UpdateWorkload in Consul. Does not alter the service if only checks have
// changed.
//
// DriverNetwork must not change between invocations for the same allocation.
func (c *ServiceClient) UpdateWorkload(old, newWorkload *serviceregistration.WorkloadServices) error {
	ops := new(operations)
	regs := new(serviceregistration.ServiceRegistrations)
	regs.Services = make(map[string]*serviceregistration.ServiceRegistration, len(newWorkload.Services))

	newIDs := make(map[string]*structs.Service, len(newWorkload.Services))
	for _, s := range newWorkload.Services {
		newIDs[serviceregistration.MakeAllocServiceID(newWorkload.AllocInfo.AllocID, newWorkload.Name(), s)] = s
	}

	// Loop over existing Services to see if they have been removed
	for _, existingSvc := range old.Services {
		existingID := serviceregistration.MakeAllocServiceID(old.AllocInfo.AllocID, old.Name(), existingSvc)
		newSvc, ok := newIDs[existingID]

		if !ok {
			// Existing service entry removed
			ops.deregServices = append(ops.deregServices, existingID)
			for _, check := range existingSvc.Checks {
				cid := MakeCheckID(existingID, check)
				ops.deregChecks = append(ops.deregChecks, cid)

				// Unwatch watched checks
				if check.TriggersRestarts() {
					c.checkWatcher.Unwatch(cid)
				}
			}
			continue
		}

		oldHash := existingSvc.Hash(old.AllocInfo.AllocID, old.Name(), old.Canary)
		newHash := newSvc.Hash(newWorkload.AllocInfo.AllocID, newWorkload.Name(), newWorkload.Canary)
		if oldHash == newHash {
			// Service exists and hasn't changed, don't re-add it later
			delete(newIDs, existingID)
		}

		// Service still exists so add it to the task's registration
		sreg := &serviceregistration.ServiceRegistration{
			ServiceID:     existingID,
			CheckIDs:      make(map[string]struct{}, len(newSvc.Checks)),
			CheckOnUpdate: make(map[string]string, len(newSvc.Checks)),
		}
		regs.Services[existingID] = sreg

		// See if any checks were updated
		existingChecks := make(map[string]*structs.ServiceCheck, len(existingSvc.Checks))
		for _, check := range existingSvc.Checks {
			existingChecks[MakeCheckID(existingID, check)] = check
		}

		// Register new checks
		for _, check := range newSvc.Checks {
			checkID := MakeCheckID(existingID, check)
			if _, exists := existingChecks[checkID]; exists {
				// Check is still required. Remove it from the map so it doesn't get
				// deleted later.
				delete(existingChecks, checkID)
				sreg.CheckIDs[checkID] = struct{}{}
				sreg.CheckOnUpdate[checkID] = check.OnUpdate
			}

			// New check on an unchanged service; add them now
			checkRegs, err := c.checkRegs(existingID, newSvc, newWorkload, sreg)
			if err != nil {
				return err
			}

			for _, registration := range checkRegs {
				sreg.CheckIDs[registration.ID] = struct{}{}
				sreg.CheckOnUpdate[registration.ID] = check.OnUpdate
				ops.regChecks = append(ops.regChecks, registration)
			}

			// Update all watched checks as CheckRestart fields aren't part of ID
			if check.TriggersRestarts() {
				c.checkWatcher.Watch(newWorkload.AllocInfo.AllocID, newWorkload.Name(), checkID, check, newWorkload.Restarter)
			}
		}

		// Remove existing checks not in updated service
		for cid, check := range existingChecks {
			ops.deregChecks = append(ops.deregChecks, cid)

			// Unwatch checks
			if check.TriggersRestarts() {
				c.checkWatcher.Unwatch(cid)
			}
		}
	}

	// Any remaining services should just be enqueued directly
	for _, newSvc := range newIDs {
		sreg, err := c.serviceRegs(ops, newSvc, newWorkload)
		if err != nil {
			return err
		}

		regs.Services[sreg.ServiceID] = sreg
	}

	// Add the task to the allocation's registration
	c.addRegistrations(newWorkload.AllocInfo.AllocID, newWorkload.Name(), regs)

	c.commit(ops)

	// Start watching checks. Done after service registrations are built
	// since an error building them could leak watches.
	for serviceID, service := range newIDs {
		for _, check := range service.Checks {
			if check.TriggersRestarts() {
				checkID := MakeCheckID(serviceID, check)
				c.checkWatcher.Watch(newWorkload.AllocInfo.AllocID, newWorkload.Name(), checkID, check, newWorkload.Restarter)
			}
		}
	}

	return nil
}

// RemoveWorkload from Consul. Removes all service entries and checks.
//
// Actual communication with Consul is done asynchronously (see Run).
func (c *ServiceClient) RemoveWorkload(workload *serviceregistration.WorkloadServices) {
	ops := operations{}

	for _, service := range workload.Services {
		id := serviceregistration.MakeAllocServiceID(workload.AllocInfo.AllocID, workload.Name(), service)
		ops.deregServices = append(ops.deregServices, id)

		for _, check := range service.Checks {
			cid := MakeCheckID(id, check)
			ops.deregChecks = append(ops.deregChecks, cid)

			if check.TriggersRestarts() {
				c.checkWatcher.Unwatch(cid)
			}
		}
	}

	// Remove the workload from the alloc's registrations
	c.removeRegistration(workload.AllocInfo.AllocID, workload.Name())

	// Now add them to the deregistration fields; main Run loop will update
	c.commit(&ops)
}

// normalizeNamespace will turn the "default" namespace into the empty string,
// so that Consul OSS will not produce an error setting something in the default
// namespace.
func normalizeNamespace(namespace string) string {
	if namespace == "default" {
		return ""
	}
	return namespace
}

// AllocRegistrations returns the registrations for the given allocation. If the
// allocation has no registrations, the response is a nil object.
func (c *ServiceClient) AllocRegistrations(allocID string) (*serviceregistration.AllocRegistration, error) {
	// Get the internal struct using the lock
	c.allocRegistrationsLock.RLock()
	regInternal, ok := c.allocRegistrations[allocID]
	if !ok {
		c.allocRegistrationsLock.RUnlock()
		return nil, nil
	}

	// Copy so we don't expose internal structs
	reg := regInternal.Copy()
	c.allocRegistrationsLock.RUnlock()

	// Get the list of all namespaces created so we can iterate them.
	namespaces, err := c.namespacesClient.List()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve namespaces from consul: %w", err)
	}

	services := make(map[string]*api.AgentService)
	checks := make(map[string]*api.AgentCheck)

	// Query the services and checks to populate the allocation registrations.
	for _, namespace := range namespaces {
		nsServices, err := c.agentAPI.ServicesWithFilterOpts("", &api.QueryOptions{Namespace: normalizeNamespace(namespace)})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve services from consul: %w", err)
		}
		for k, v := range nsServices {
			services[k] = v
		}

		nsChecks, err := c.agentAPI.ChecksWithFilterOpts("", &api.QueryOptions{Namespace: normalizeNamespace(namespace)})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve checks from consul: %w", err)
		}
		for k, v := range nsChecks {
			checks[k] = v
		}
	}

	// Populate the object
	for _, treg := range reg.Tasks {
		for serviceID, sreg := range treg.Services {
			sreg.Service = services[serviceID]
			for checkID := range sreg.CheckIDs {
				if check, ok := checks[checkID]; ok {
					sreg.Checks = append(sreg.Checks, check)
				}
			}
		}
	}

	return reg, nil
}

// UpdateTTL is used to update the TTL of a check. Typically this will only be
// called to heartbeat script checks.
func (c *ServiceClient) UpdateTTL(id, namespace, output, status string) error {
	ns := normalizeNamespace(namespace)
	return c.agentAPI.UpdateTTLOpts(id, output, status, &api.QueryOptions{Namespace: ns})
}

// Shutdown the Consul client. Update running task registrations and deregister
// agent from Consul. On first call blocks up to shutdownWait before giving up
// on syncing operations.
func (c *ServiceClient) Shutdown() error {
	// Serialize Shutdown calls with RegisterAgent to prevent leaking agent
	// entries.
	c.agentLock.Lock()
	defer c.agentLock.Unlock()
	select {
	case <-c.shutdownCh:
		return nil
	default:
		close(c.shutdownCh)
	}

	// Give run loop time to sync, but don't block indefinitely
	deadline := time.After(c.shutdownWait)

	// Wait for Run to finish any outstanding operations and exit
	select {
	case <-c.exitCh:
	case <-deadline:
		// Don't wait forever though
	}

	// If Consul was never seen nothing could be written so exit early
	if !c.hasSeen() {
		return nil
	}

	// Always attempt to deregister Nomad agent Consul entries, even if
	// deadline was reached
	for _, id := range c.agentServices.List() {
		if err := c.agentAPI.ServiceDeregisterOpts(id, nil); err != nil {
			c.logger.Error("failed deregistering agent service", "service_id", id, "error", err)
		}
	}

	namespaces, err := c.namespacesClient.List()
	if err != nil {
		c.logger.Error("failed to retrieve namespaces from consul", "error", err)
	}

	remainingChecks := make(map[string]*api.AgentCheck)
	for _, namespace := range namespaces {
		nsChecks, err := c.agentAPI.ChecksWithFilterOpts("", &api.QueryOptions{Namespace: normalizeNamespace(namespace)})
		if err != nil {
			c.logger.Error("failed to retrieve checks from consul", "error", err)
		}
		for k, v := range nsChecks {
			remainingChecks[k] = v
		}
	}

	checkRemains := func(id string) bool {
		for _, c := range remainingChecks {
			if c.CheckID == id {
				return true
			}
		}
		return false
	}

	for _, id := range c.agentChecks.List() {
		// if we couldn't populate remainingChecks it is unlikely that CheckDeregister will work, but try anyway
		// if we could list the remaining checks, verify that the check we store still exists before removing it.
		if remainingChecks == nil || checkRemains(id) {
			ns := remainingChecks[id].Namespace
			if err := c.agentAPI.CheckDeregisterOpts(id, &api.QueryOptions{Namespace: ns}); err != nil {
				c.logger.Error("failed deregistering agent check", "check_id", id, "error", err)
			}
		}
	}

	return nil
}

// addRegistration adds the service registrations for the given allocation.
func (c *ServiceClient) addRegistrations(allocID, taskName string, reg *serviceregistration.ServiceRegistrations) {
	c.allocRegistrationsLock.Lock()
	defer c.allocRegistrationsLock.Unlock()

	alloc, ok := c.allocRegistrations[allocID]
	if !ok {
		alloc = &serviceregistration.AllocRegistration{
			Tasks: make(map[string]*serviceregistration.ServiceRegistrations),
		}
		c.allocRegistrations[allocID] = alloc
	}
	alloc.Tasks[taskName] = reg
}

// removeRegistrations removes the registration for the given allocation.
func (c *ServiceClient) removeRegistration(allocID, taskName string) {
	c.allocRegistrationsLock.Lock()
	defer c.allocRegistrationsLock.Unlock()

	alloc, ok := c.allocRegistrations[allocID]
	if !ok {
		return
	}

	// Delete the task and if it is the last one also delete the alloc's
	// registration
	delete(alloc.Tasks, taskName)
	if len(alloc.Tasks) == 0 {
		delete(c.allocRegistrations, allocID)
	}
}

// makeAgentServiceID creates a unique ID for identifying an agent service in
// Consul.
//
// Agent service IDs are of the form:
//
//	{nomadServicePrefix}-{ROLE}-b32(sha1({Service.Name}-{Service.Tags...})
//	Example Server ID: _nomad-server-fbbk265qn4tmt25nd4ep42tjvmyj3hr4
//	Example Client ID: _nomad-client-ggnjpgl7yn7rgmvxzilmpvrzzvrszc7l
func makeAgentServiceID(role string, service *structs.Service) string {
	return fmt.Sprintf("%s-%s-%s", nomadServicePrefix, role, service.Hash(role, "", false))
}

// MakeCheckID creates a unique ID for a check.
//
//	Example Check ID: _nomad-check-434ae42f9a57c5705344974ac38de2aee0ee089d
func MakeCheckID(serviceID string, check *structs.ServiceCheck) string {
	return fmt.Sprintf("%s%s", nomadCheckPrefix, check.Hash(serviceID))
}

// createCheckReg creates a Check that can be registered with Consul.
//
// Script checks simply have a TTL set and the caller is responsible for
// running the script and heart-beating.
func createCheckReg(serviceID, checkID string, check *structs.ServiceCheck, host string, port int, namespace string) (*api.AgentCheckRegistration, error) {
	chkReg := api.AgentCheckRegistration{
		ID:        checkID,
		Name:      check.Name,
		ServiceID: serviceID,
		Namespace: normalizeNamespace(namespace),
	}
	chkReg.Status = check.InitialStatus
	chkReg.Timeout = check.Timeout.String()
	chkReg.Interval = check.Interval.String()
	chkReg.SuccessBeforePassing = check.SuccessBeforePassing
	chkReg.FailuresBeforeCritical = check.FailuresBeforeCritical

	// Require an address for http or tcp checks
	if port == 0 && check.RequiresPort() {
		return nil, fmt.Errorf("%s checks require an address", check.Type)
	}

	switch check.Type {
	case structs.ServiceCheckHTTP:
		proto := check.Protocol
		if proto == "" {
			proto = "http"
		}
		if check.TLSSkipVerify {
			chkReg.TLSSkipVerify = true
		}
		chkReg.TLSServerName = check.TLSServerName
		base := url.URL{
			Scheme: proto,
			Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		}
		relative, err := url.Parse(check.Path)
		if err != nil {
			return nil, err
		}
		checkURL := base.ResolveReference(relative)
		chkReg.HTTP = checkURL.String()
		chkReg.Method = check.Method
		chkReg.Header = check.Header
		chkReg.Body = check.Body

	case structs.ServiceCheckTCP:
		chkReg.TCP = net.JoinHostPort(host, strconv.Itoa(port))

	case structs.ServiceCheckScript:
		chkReg.TTL = (check.Interval + ttlCheckBuffer).String()
		// As of Consul 1.0.0 setting TTL and Interval is a 400
		chkReg.Interval = ""

	case structs.ServiceCheckGRPC:
		chkReg.GRPC = fmt.Sprintf("%s/%s", net.JoinHostPort(host, strconv.Itoa(port)), check.GRPCService)
		chkReg.GRPCUseTLS = check.GRPCUseTLS
		if check.TLSSkipVerify {
			chkReg.TLSSkipVerify = true
		}
		chkReg.TLSServerName = check.TLSServerName

	default:
		return nil, fmt.Errorf("check type %+q not valid", check.Type)
	}
	return &chkReg, nil
}

// isNomadClient returns true if id represents a Nomad Client registration.
func isNomadClient(id string) bool {
	return strings.HasPrefix(id, nomadClientPrefix)
}

// isNomadServer returns true if id represents a Nomad Server registration.
func isNomadServer(id string) bool {
	return strings.HasPrefix(id, nomadServerPrefix)
}

// isNomadAgent returns true if id represents a Nomad Client or Server registration.
func isNomadAgent(id string) bool {
	return isNomadClient(id) || isNomadServer(id)
}

// isNomadService returns true if the ID matches the pattern of a Nomad managed
// service (new or old formats). Agent services return false as independent
// client and server agents may be running on the same machine. #2827
func isNomadService(id string) bool {
	return strings.HasPrefix(id, nomadTaskPrefix) || isOldNomadService(id)
}

// isNomadCheck returns true if the ID matches the pattern of a Nomad managed
// check.
func isNomadCheck(id string) bool {
	return strings.HasPrefix(id, nomadCheckPrefix)
}

// isOldNomadService returns true if the ID matches an old pattern managed by
// Nomad.
//
// Pre-0.7.1 task service IDs are of the form:
//
//	{nomadServicePrefix}-executor-{ALLOC_ID}-{Service.Name}-{Service.Tags...}
//	Example Service ID: _nomad-executor-1234-echo-http-tag1-tag2-tag3
func isOldNomadService(id string) bool {
	const prefix = nomadServicePrefix + "-executor"
	return strings.HasPrefix(id, prefix)
}

const (
	sidecarSuffix = "-sidecar-proxy"
)

// maybeConnectSidecar returns true if the ID is likely of a Connect sidecar proxy.
// This function should only be used to determine if Nomad should skip managing
// service id; it could produce false negatives for non-Nomad managed services
// (i.e. someone set the ID manually), but Nomad does not manage those anyway.
//
// It is important not to reference the parent service, which may or may not still
// be tracked by Nomad internally.
//
// For example if you have a Connect enabled service with the ID:
//
//	_nomad-task-5229c7f8-376b-3ccc-edd9-981e238f7033-cache-redis-cache-db
//
// Consul will create a service for the sidecar proxy with the ID:
//
//	_nomad-task-5229c7f8-376b-3ccc-edd9-981e238f7033-cache-redis-cache-db-sidecar-proxy
func maybeConnectSidecar(id string) bool {
	return strings.HasSuffix(id, sidecarSuffix)
}

var (
	sidecarProxyCheckRe = regexp.MustCompile(`^service:_nomad-.+-sidecar-proxy(:[\d]+)?$`)
)

// maybeSidecarProxyCheck returns true if the ID likely matches a Nomad generated
// check ID used in the context of a Nomad managed Connect sidecar proxy. This function
// should only be used to determine if Nomad should skip managing a check; it can
// produce false negatives for non-Nomad managed Connect sidecar proxy checks (i.e.
// someone set the ID manually), but Nomad does not manage those anyway.
//
// For example if you have a Connect enabled service with the ID:
//
//	_nomad-task-5229c7f8-376b-3ccc-edd9-981e238f7033-cache-redis-cache-db
//
// Nomad will create a Connect sidecar proxy of ID:
//
// _nomad-task-5229c7f8-376b-3ccc-edd9-981e238f7033-cache-redis-cache-db-sidecar-proxy
//
// With default checks like:
//
// service:_nomad-task-2f5fb517-57d4-44ee-7780-dc1cb6e103cd-group-api-count-api-9001-sidecar-proxy:1
// service:_nomad-task-2f5fb517-57d4-44ee-7780-dc1cb6e103cd-group-api-count-api-9001-sidecar-proxy:2
//
// Unless sidecar_service.disable_default_tcp_check is set, in which case the
// default check is:
//
// service:_nomad-task-322616db-2680-35d8-0d10-b50a0a0aa4cd-group-api-count-api-9001-sidecar-proxy
func maybeSidecarProxyCheck(id string) bool {
	return sidecarProxyCheckRe.MatchString(id)
}

// getNomadSidecar returns the service registration of the sidecar for the managed
// service with the specified id.
//
// If the managed service of the specified id does not exist, or the service does
// not have a sidecar proxy, nil is returned.
func getNomadSidecar(id string, services map[string]*api.AgentService) *api.AgentService {
	if _, exists := services[id]; !exists {
		return nil
	}

	sidecarID := id + sidecarSuffix
	return services[sidecarID]
}

func parseAddress(raw string, port int) (api.ServiceAddress, error) {
	result := api.ServiceAddress{}
	addr, portStr, err := net.SplitHostPort(raw)
	// Error message from Go's net/ipsock.go
	if err != nil {
		if !strings.Contains(err.Error(), "missing port in address") {
			return result, fmt.Errorf("error parsing address %q: %v", raw, err)
		}

		// Use the whole input as the address if there wasn't a port.
		if ip := net.ParseIP(raw); ip == nil {
			return result, fmt.Errorf("error parsing address %q: not an IP address", raw)
		}
		addr = raw
	}

	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return result, fmt.Errorf("error parsing port %q: %v", portStr, err)
		}
	}

	result.Address = addr
	result.Port = port
	return result, nil
}

// morph the tagged_addresses map into the structure consul api wants
func parseTaggedAddresses(m map[string]string, port int) (map[string]api.ServiceAddress, error) {
	result := make(map[string]api.ServiceAddress, len(m))
	for k, v := range m {
		sa, err := parseAddress(v, port)
		if err != nil {
			return nil, err
		}
		result[k] = sa
	}
	return result, nil
}
