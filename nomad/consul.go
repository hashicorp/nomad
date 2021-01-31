package nomad

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

const (
	// siTokenDescriptionFmt is the format for the .Description field of
	// service identity tokens generated on behalf of Nomad.
	siTokenDescriptionFmt = "_nomad_si [%s] [%s] [%s]"

	// siTokenRequestRateLimit is the maximum number of requests per second Nomad
	// will make against Consul for requesting SI tokens.
	siTokenRequestRateLimit rate.Limit = 500

	// siTokenMaxParallelRevokes is the maximum number of parallel SI token
	// revocation requests Nomad will make against Consul.
	siTokenMaxParallelRevokes = 64

	// siTokenRevocationInterval is the interval at which SI tokens that failed
	// initial revocation are retried.
	siTokenRevocationInterval = 5 * time.Minute
)

const (
	// configEntriesRequestRateLimit is the maximum number of requests per second
	// Nomad will make against Consul for operations on global Configuration Entry
	// objects.
	configEntriesRequestRateLimit rate.Limit = 10
)

const (
	// ConsulPolicyWrite is the literal text of the policy field of a Consul Policy
	// Rule that we check when validating an Operator Consul token against the
	// necessary permissions for creating a Service Identity token for a given
	// service.
	//
	// The rule may be:
	//  - service.<exact>
	//  - service."*" (wildcard)
	//  - service_prefix.<matching> (including empty string)
	//
	// e.g.
	//   service "web" { policy = "write" }
	//   service_prefix "" { policy = "write" }
	ConsulPolicyWrite = "write"
)

type ServiceIdentityRequest struct {
	TaskKind  structs.TaskKind
	TaskName  string
	ClusterID string
	AllocID   string
}

func (sir ServiceIdentityRequest) Validate() error {
	switch {
	case sir.ClusterID == "":
		return errors.New("cluster id not set")
	case sir.AllocID == "":
		return errors.New("alloc id not set")
	case sir.TaskName == "":
		return errors.New("task name not set")
	case sir.TaskKind == "":
		return errors.New("task kind not set")
	default:
		return nil
	}
}

func (sir ServiceIdentityRequest) Description() string {
	return fmt.Sprintf(siTokenDescriptionFmt, sir.ClusterID, sir.AllocID, sir.TaskName)
}

// ConsulACLsAPI is an abstraction over the consul/api.ACL API used by Nomad
// Server.
//
// ACL requirements
// - acl:write (transitive through ACLsAPI)
type ConsulACLsAPI interface {

	// CheckSIPolicy checks that the given operator token has the equivalent ACL
	// permissiveness that a Service Identity token policy for task would have.
	CheckSIPolicy(ctx context.Context, task, secretID string) error

	// Create instructs Consul to create a Service Identity token.
	CreateToken(context.Context, ServiceIdentityRequest) (*structs.SIToken, error)

	// RevokeTokens instructs Consul to revoke the given token accessors.
	RevokeTokens(context.Context, []*structs.SITokenAccessor, bool) bool

	// MarkForRevocation marks the tokens for background revocation
	MarkForRevocation([]*structs.SITokenAccessor)

	// Stop is used to stop background token revocations. Intended to be used
	// on Nomad Server shutdown.
	Stop()

	// todo(shoenig): use list endpoint for finding orphaned tokens
	// ListTokens lists every token in Consul.
	// ListTokens() ([]string, error)
}

// PurgeSITokenAccessorFunc is called to remove SI Token accessors from the
// system (i.e. raft). If the function returns an error, the token will still
// be tracked and revocation attempts will retry in the background until there
// is a success.
type PurgeSITokenAccessorFunc func([]*structs.SITokenAccessor) error

type SITokenStats struct {
	TrackedForRevoke int
}

type consulACLsAPI struct {
	// aclClient is the API subset of the real consul client we need for
	// managing Service Identity tokens
	aclClient consul.ACLsAPI

	// limiter is used to rate limit requests to consul
	limiter *rate.Limiter

	bgRevokeLock sync.Mutex
	// Track accessors that must have their revocation retried in the background.
	bgRetryRevocation []*structs.SITokenAccessor
	// Track whether the background revocations have been stopped, to avoid
	// creating tokens we would no longer be able to revoke. Expected to be used
	// on a Server shutdown.
	bgRevokeStopped bool

	// purgeFunc is the Nomad Server function that removes the reference to the
	// SI token accessor from the persistent raft store
	purgeFunc PurgeSITokenAccessorFunc

	// stopC is used to signal the client is shutting down and token revocation
	// background goroutine should stop
	stopC chan struct{}

	// logger is used to log messages
	logger hclog.Logger
}

func NewConsulACLsAPI(aclClient consul.ACLsAPI, logger hclog.Logger, purgeFunc PurgeSITokenAccessorFunc) *consulACLsAPI {
	if purgeFunc == nil {
		purgeFunc = func([]*structs.SITokenAccessor) error { return nil }
	}

	c := &consulACLsAPI{
		aclClient: aclClient,
		limiter:   rate.NewLimiter(siTokenRequestRateLimit, int(siTokenRequestRateLimit)),
		stopC:     make(chan struct{}),
		purgeFunc: purgeFunc,
		logger:    logger.Named("consul_acl"),
	}

	go c.bgRetryRevokeDaemon()

	return c
}

// Stop stops background token revocations from happening. Once stopped, tokens
// may no longer be created.
func (c *consulACLsAPI) Stop() {
	c.bgRevokeLock.Lock()
	defer c.bgRevokeLock.Unlock()

	c.stopC <- struct{}{}
	c.bgRevokeStopped = true
}

func (c *consulACLsAPI) CheckSIPolicy(ctx context.Context, task, secretID string) error {
	defer metrics.MeasureSince([]string{"nomad", "consul", "check_si_policy"}, time.Now())

	if id := strings.TrimSpace(secretID); id == "" {
		return errors.New("missing consul token")
	}

	// Ensure we are under our rate limit.
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}

	opToken, _, err := c.aclClient.TokenReadSelf(&api.QueryOptions{
		AllowStale: false,
		Token:      secretID,
	})
	if err != nil {
		return errors.Wrap(err, "unable to validate operator consul token")
	}

	allowable, err := c.hasSufficientPolicy(task, opToken)
	if err != nil {
		return errors.Wrap(err, "unable to validate operator consul token")
	}
	if !allowable {
		return errors.Errorf("permission denied for %q", task)
	}

	return nil
}

func (c *consulACLsAPI) CreateToken(ctx context.Context, sir ServiceIdentityRequest) (*structs.SIToken, error) {
	defer metrics.MeasureSince([]string{"nomad", "consul", "create_token"}, time.Now())

	// make sure the background token revocations have not been stopped
	c.bgRevokeLock.Lock()
	stopped := c.bgRevokeStopped
	c.bgRevokeLock.Unlock()

	if stopped {
		return nil, errors.New("client stopped and may no longer create tokens")
	}

	// sanity check the metadata for the token we want
	if err := sir.Validate(); err != nil {
		return nil, err
	}

	// the SI token created must be for the service, not the sidecar of the service
	// https://www.consul.io/docs/acl/acl-system.html#acl-service-identities
	service := sir.TaskKind.Value()
	partial := &api.ACLToken{
		Description:       sir.Description(),
		ServiceIdentities: []*api.ACLServiceIdentity{{ServiceName: service}},
	}

	// Ensure we are under our rate limit.
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	token, _, err := c.aclClient.TokenCreate(partial, nil)
	if err != nil {
		return nil, err
	}

	return &structs.SIToken{
		TaskName:   sir.TaskName,
		AccessorID: token.AccessorID,
		SecretID:   token.SecretID,
	}, nil
}

// RevokeTokens revokes the passed set of SI token accessors. If committed is set,
// the client's purge function is called (which purges the tokens from the Server's
// persistent store). If there is an error purging either because of Consul failures
// or because of the purge function, the revocation is retried in the background.
//
// The revocation of an SI token accessor is idempotent.
//
// A return value of true indicates one or more accessors were stored for
// a revocation retry attempt in the background (intended for tests).
func (c *consulACLsAPI) RevokeTokens(ctx context.Context, accessors []*structs.SITokenAccessor, committed bool) bool {
	defer metrics.MeasureSince([]string{"nomad", "consul", "revoke_tokens"}, time.Now())

	nTokens := float32(len(accessors))

	if err := c.parallelRevoke(ctx, accessors); err != nil {
		// If these tokens were uncommitted into raft, it is a best effort to
		// revoke them now. If this immediate revocation does not work, Nomad loses
		// track of them and will need to do a brute reconciliation later. This
		// should happen rarely, and will be implemented soon.
		if !committed {
			metrics.IncrCounter([]string{"nomad", "consul", "undistributed_si_tokens_abandoned"}, nTokens)
		}

		c.logger.Warn("failed to revoke tokens, will reattempt later", "error", err)
		c.storeForRevocation(accessors)
		return true
	}

	if !committed {
		// Un-committed tokens were revoked without incident (nothing to purge)
		metrics.IncrCounter([]string{"nomad", "consul", "undistributed_si_tokens_revoked"}, nTokens)
		return false
	}

	// Committed tokens were revoked without incident, now purge them
	if err := c.purgeFunc(accessors); err != nil {
		c.logger.Error("failed to purge SI token accessors", "error", err)
		c.storeForRevocation(accessors)
		return true
	}

	// Track that the SI tokens were revoked and purged successfully
	metrics.IncrCounter([]string{"nomad", "consul", "distributed_si_tokens_revoked"}, nTokens)
	return false
}

func (c *consulACLsAPI) MarkForRevocation(accessors []*structs.SITokenAccessor) {
	c.storeForRevocation(accessors)
}

func (c *consulACLsAPI) storeForRevocation(accessors []*structs.SITokenAccessor) {
	c.bgRevokeLock.Lock()
	defer c.bgRevokeLock.Unlock()

	// copy / append the set of accessors we must track for revocation in the
	// background
	c.bgRetryRevocation = append(c.bgRetryRevocation, accessors...)
}

func (c *consulACLsAPI) parallelRevoke(ctx context.Context, accessors []*structs.SITokenAccessor) error {
	g, pCtx := errgroup.WithContext(ctx)

	// Cap the handlers
	handlers := len(accessors)
	if handlers > siTokenMaxParallelRevokes {
		handlers = siTokenMaxParallelRevokes
	}

	// Revoke the SI Token Accessors
	input := make(chan *structs.SITokenAccessor, handlers)
	for i := 0; i < handlers; i++ {
		g.Go(func() error {
			for {
				select {
				case accessor, ok := <-input:
					if !ok {
						return nil
					}
					if err := c.singleRevoke(ctx, accessor); err != nil {
						return errors.Wrapf(err,
							"failed to revoke SI token accessor (alloc %q, node %q, task %q)",
							accessor.AllocID, accessor.NodeID, accessor.TaskName,
						)
					}
				case <-pCtx.Done():
					return nil
				}
			}
		})
	}

	// Send the input
	go func() {
		defer close(input)
		for _, accessor := range accessors {
			select {
			case <-pCtx.Done():
				return
			case input <- accessor:
			}
		}
	}()

	// Wait for everything to complete
	return g.Wait()
}

func (c *consulACLsAPI) singleRevoke(ctx context.Context, accessor *structs.SITokenAccessor) error {
	c.logger.Trace("revoke SI token", "task", accessor.TaskName, "alloc_id", accessor.AllocID, "node_id", accessor.NodeID)

	// Ensure we are under our rate limit.
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}

	// Consul will no-op the deletion of a non-existent token (no error)
	_, err := c.aclClient.TokenDelete(accessor.AccessorID, nil)
	return err
}

func (c *consulACLsAPI) bgRetryRevokeDaemon() {
	ticker := time.NewTicker(siTokenRevocationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopC:
			return
		case <-ticker.C:
			c.bgRetryRevoke()
		}
	}
}

// maxConsulRevocationBatchSize is the maximum tokens a bgRetryRevoke should revoke
// at any given time.
const maxConsulRevocationBatchSize = 1000

func (c *consulACLsAPI) bgRetryRevoke() {
	c.bgRevokeLock.Lock()
	defer c.bgRevokeLock.Unlock()

	// fast path, nothing to do
	if len(c.bgRetryRevocation) == 0 {
		return
	}

	// unlike vault tokens, SI tokens do not have a TTL, and so we must try to
	// remove all SI token accessors, every time, until they're gone
	toRevoke := len(c.bgRetryRevocation)
	if toRevoke > maxConsulRevocationBatchSize {
		toRevoke = maxConsulRevocationBatchSize
	}
	toPurge := make([]*structs.SITokenAccessor, toRevoke)
	copy(toPurge, c.bgRetryRevocation)

	if err := c.parallelRevoke(context.Background(), toPurge); err != nil {
		c.logger.Warn("background SI token revocation failed", "error", err)
		return
	}

	// Call the revocation function
	if err := c.purgeFunc(toPurge); err != nil {
		// Just try again later (revocation is idempotent)
		c.logger.Error("background SI token purge failed", "error", err)
		return
	}

	// Track that the SI tokens were revoked successfully
	nTokens := float32(len(toPurge))
	metrics.IncrCounter([]string{"nomad", "consul", "distributed_tokens_revoked"}, nTokens)

	// Reset the list of accessors to retry, since we just removed them all.
	c.bgRetryRevocation = nil
}

func (c *consulACLsAPI) ListTokens() ([]string, error) {
	// defer metrics.MeasureSince([]string{"nomad", "consul", "list_tokens"}, time.Now())
	return nil, errors.New("not yet implemented")
}

// purgeSITokenAccessors is the Nomad Server method which will remove the set
// of SI token accessors from the persistent raft store.
func (s *Server) purgeSITokenAccessors(accessors []*structs.SITokenAccessor) error {
	// Commit this update via Raft
	request := structs.SITokenAccessorsRequest{Accessors: accessors}
	_, _, err := s.raftApply(structs.ServiceIdentityAccessorDeregisterRequestType, request)
	return err
}

// ConsulConfigsAPI is an abstraction over the consul/api.ConfigEntries API used by
// Nomad Server.
//
// Nomad will only perform write operations on Consul Ingress Gateway Configuration Entries.
// Removing the entries is not particularly safe, given that multiple Nomad clusters
// may be writing to the same config entries, which are global in the Consul scope.
type ConsulConfigsAPI interface {
	// SetIngressCE adds the given ConfigEntry to Consul, overwriting
	// the previous entry if set.
	SetIngressCE(ctx context.Context, service string, entry *structs.ConsulIngressConfigEntry) error

	// SetTerminatingCE adds the given ConfigEntry to Consul, overwriting
	// the previous entry if set.
	SetTerminatingCE(ctx context.Context, service string, entry *structs.ConsulTerminatingConfigEntry) error

	// Stop is used to stop additional creations of Configuration Entries. Intended to
	// be used on Nomad Server shutdown.
	Stop()
}

type consulConfigsAPI struct {
	// configsClient is the API subset of the real Consul client we need for
	// managing Configuration Entries.
	configsClient consul.ConfigAPI

	// limiter is used to rate limit requests to Consul
	limiter *rate.Limiter

	// logger is used to log messages
	logger hclog.Logger

	// lock protects the stopped flag, which prevents use of the consul configs API
	// client after shutdown.
	lock    sync.Mutex
	stopped bool
}

func NewConsulConfigsAPI(configsClient consul.ConfigAPI, logger hclog.Logger) *consulConfigsAPI {
	return &consulConfigsAPI{
		configsClient: configsClient,
		limiter:       rate.NewLimiter(configEntriesRequestRateLimit, int(configEntriesRequestRateLimit)),
		logger:        logger,
	}
}

func (c *consulConfigsAPI) Stop() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.stopped = true
}

func (c *consulConfigsAPI) SetIngressCE(ctx context.Context, service string, entry *structs.ConsulIngressConfigEntry) error {
	return c.setCE(ctx, convertIngressCE(service, entry))
}

func (c *consulConfigsAPI) SetTerminatingCE(ctx context.Context, service string, entry *structs.ConsulTerminatingConfigEntry) error {
	return c.setCE(ctx, convertTerminatingCE(service, entry))
}

// also mesh

// setCE will set the Configuration Entry of any type Consul supports.
func (c *consulConfigsAPI) setCE(ctx context.Context, entry api.ConfigEntry) error {
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

	_, _, err := c.configsClient.Set(entry, nil)
	return err
}

func convertIngressCE(service string, entry *structs.ConsulIngressConfigEntry) api.ConfigEntry {
	var listeners []api.IngressListener = nil
	for _, listener := range entry.Listeners {
		var services []api.IngressService = nil
		for _, s := range listener.Services {
			services = append(services, api.IngressService{
				Name:  s.Name,
				Hosts: helper.CopySliceString(s.Hosts),
			})
		}
		listeners = append(listeners, api.IngressListener{
			Port:     listener.Port,
			Protocol: listener.Protocol,
			Services: services,
		})
	}

	tlsEnabled := false
	if entry.TLS != nil && entry.TLS.Enabled {
		tlsEnabled = true
	}

	return &api.IngressGatewayConfigEntry{
		Kind:      api.IngressGateway,
		Name:      service,
		TLS:       api.GatewayTLSConfig{Enabled: tlsEnabled},
		Listeners: listeners,
	}
}

func convertTerminatingCE(service string, entry *structs.ConsulTerminatingConfigEntry) api.ConfigEntry {
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
		Kind:     api.TerminatingGateway,
		Name:     service,
		Services: linked,
	}
}
