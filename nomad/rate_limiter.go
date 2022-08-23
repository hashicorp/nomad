package nomad

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// CheckRateLimit finds the appropriate limiter for this endpoint and
// operation and returns ErrTooManyRequests if the rate limit has been
// exceeded
func (srv *Server) CheckRateLimit(endpoint, op, secretID string, rpcCtx *RPCContext) error {
	if !srv.config.ACLEnabled || rpcCtx == nil {
		// if the rpcCtx isn't set, we're the RPC caller and not the server
		return nil
	}

	srv.rpcRateLimiter.lock.RLock()
	defer srv.rpcRateLimiter.lock.RUnlock()

	// if this is nil it's a programming error, so just let it blow up
	limiter := srv.rpcRateLimiter.limiters[endpoint]

	accessor, err := srv.accessorFromSecretID(secretID, rpcCtx)
	if err != nil {
		return err
	}

	return limiter.check(srv.shutdownCtx, op, accessor)
}

func (srv *Server) accessorFromSecretID(secretID string, rpcCtx *RPCContext) (string, error) {

	// get the user ACLToken or anonymous token
	token, err := srv.ResolveSecretToken(secretID)

	switch err {
	case structs.ErrTokenNotFound:
		if secretID == srv.getLeaderAcl() {
			return fmt.Sprintf("leader=%s", srv.config.NodeID), nil
		}
		node, err := srv.State().NodeBySecretID(nil, secretID)
		if err != nil {
			return "", fmt.Errorf("could not resolve rate limit user: %v", err)
		}
		if node != nil {
			return fmt.Sprintf("node=%s", node.ID), nil
		}
		claims, err := srv.VerifyClaim(secretID)
		if err == nil {
			// unlike the state queries, errors here are invalid tokens
			return fmt.Sprintf("alloc=%s", claims.AllocationID), nil
		}

	case nil:
		if token != nil && token.AccessorID != structs.AnonymousACLToken.AccessorID {
			return fmt.Sprintf("acl=%s", token.AccessorID), nil
		}

	default:
		return "", fmt.Errorf("could not resolve rate limit user: %v", err)

	}

	// At this point we have an anonymous token or an invalid one; fall back to
	// the connection NodeID or connection address
	//
	// TODO: for server-to-server connections this currently seems to be as good
	// as we can get? Would be nice to have the server ID in the connection

	if rpcCtx.NodeID != "" {
		return fmt.Sprintf("node=%s", rpcCtx.NodeID), nil
	}
	if rpcCtx.Session != nil {
		return fmt.Sprintf("addr=%s", rpcCtx.Session.RemoteAddr().String()), nil
	}
	if rpcCtx.Conn != nil {
		return fmt.Sprintf("addr=%s", rpcCtx.Conn.RemoteAddr().String()), nil
	}

	// TODO: probably a programmer error if we hit this?
	srv.logger.Error("could not resolve any rate limit key from auth token or connection")
	return "unknown", nil
}

// RateLimiter holds all the rate limiting state
type RateLimiter struct {
	shutdownCtx context.Context
	lock        sync.RWMutex
	limiters    map[string]*endpointLimiter
}

func newRateLimiter(shutdownCtx context.Context, cfg *config.Limits) *RateLimiter {
	rl := &RateLimiter{
		shutdownCtx: shutdownCtx,
		limiters:    map[string]*endpointLimiter{},
	}
	cfg = cfg.Canonicalize()

	rl.limiters["ACL"] = newEndpointLimiter("acls", cfg.Endpoints.ACL, *cfg)
	rl.limiters["Alloc"] = newEndpointLimiter("allocs", cfg.Endpoints.Alloc, *cfg)
	rl.limiters["CSIPlugin"] = newEndpointLimiter("csi_plugin", cfg.Endpoints.CSIPlugin, *cfg)
	rl.limiters["CSIVolume"] = newEndpointLimiter("csi_volume", cfg.Endpoints.CSIVolume, *cfg)
	rl.limiters["Deployment"] = newEndpointLimiter("deployments", cfg.Endpoints.Deployment, *cfg)
	rl.limiters["Eval"] = newEndpointLimiter("evals", cfg.Endpoints.Eval, *cfg)
	rl.limiters["Job"] = newEndpointLimiter("jobs", cfg.Endpoints.Job, *cfg)
	rl.limiters["Keyring"] = newEndpointLimiter("keyring", cfg.Endpoints.Keyring, *cfg)
	rl.limiters["Namespace"] = newEndpointLimiter("namespaces", cfg.Endpoints.Namespace, *cfg)
	rl.limiters["Operator"] = newEndpointLimiter("operator", cfg.Endpoints.Operator, *cfg)
	rl.limiters["Node"] = newEndpointLimiter("nodes", cfg.Endpoints.Node, *cfg)
	rl.limiters["Periodic"] = newEndpointLimiter("periodic", cfg.Endpoints.Periodic, *cfg)
	rl.limiters["Plan"] = newEndpointLimiter("plan", cfg.Endpoints.Plan, *cfg)
	rl.limiters["Regions"] = newEndpointLimiter("regions", cfg.Endpoints.Regions, *cfg)
	rl.limiters["Scaling"] = newEndpointLimiter("scaling", cfg.Endpoints.Scaling, *cfg)
	rl.limiters["Search"] = newEndpointLimiter("search", cfg.Endpoints.Search, *cfg)
	rl.limiters["SecureVariables"] = newEndpointLimiter(
		"secure_variables", cfg.Endpoints.SecureVariables, *cfg)
	rl.limiters["ServiceRegistration"] = newEndpointLimiter(
		"services", cfg.Endpoints.ServiceRegistration, *cfg)
	rl.limiters["Status"] = newEndpointLimiter("status", cfg.Endpoints.Status, *cfg)
	rl.limiters["System"] = newEndpointLimiter("system", cfg.Endpoints.System, *cfg)

	// Enterprise-only rate limits; if this server isn't a Nomad Enterprise they are no-op
	rl.limiters["License"] = newEndpointLimiter("license", cfg.Endpoints.License, *cfg)
	rl.limiters["Quota"] = newEndpointLimiter("quotas", cfg.Endpoints.Quota, *cfg)
	rl.limiters["Recommendation"] = newEndpointLimiter("quotas", cfg.Endpoints.Recommendation, *cfg)
	rl.limiters["Sentinel"] = newEndpointLimiter("sentinel", cfg.Endpoints.Sentinel, *cfg)

	go func() {
		<-shutdownCtx.Done()
		rl.close()
	}()

	return rl
}

func (rl *RateLimiter) close() {
	rl.lock.Lock()
	defer rl.lock.Unlock()

	// we're already shutting down so provide only a short timeout on
	// this to make sure we don't hang on shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	for _, limiter := range rl.limiters {
		limiter.close(ctx)
	}
}

type endpointLimiter struct {
	endpoint string
	write    limiter.Store
	read     limiter.Store
	list     limiter.Store
}

func newEndpointLimiter(endpoint string, limits *config.RPCEndpointLimits, defaults config.Limits) *endpointLimiter {

	orElse := func(in *int, defaultVal uint64) uint64 {
		if in == nil || *in < 1 {
			return defaultVal
		}
		return uint64(*in)
	}

	write := orElse(defaults.RPCDefaultWriteRate, math.MaxUint64)
	read := orElse(defaults.RPCDefaultReadRate, math.MaxUint64)
	list := orElse(defaults.RPCDefaultListRate, math.MaxUint64)

	if limits != nil {
		write = orElse(limits.RPCWriteRate, write)
		read = orElse(limits.RPCReadRate, read)
		list = orElse(limits.RPCListRate, list)
	}

	return &endpointLimiter{
		endpoint: endpoint,
		write:    newRateLimiterStore(write),
		read:     newRateLimiterStore(read),
		list:     newRateLimiterStore(list),
	}
}

func (r *endpointLimiter) check(ctx context.Context, op, key string) error {
	var tokens, remaining uint64
	var ok bool
	var err error

	switch op {
	case acl.PolicyWrite:
		tokens, remaining, _, ok, err = r.write.Take(ctx, key)
	case acl.PolicyRead:
		tokens, remaining, _, ok, err = r.read.Take(ctx, key)
	case acl.PolicyList:
		tokens, remaining, _, ok, err = r.list.Take(ctx, key)
	default:
		// this is a programmer error, most likely because we don't
		// have real enums and it's easy to swap the two strings
		return fmt.Errorf("no such operation %q", op)
	}
	used := tokens - remaining
	metrics.IncrCounterWithLabels(
		[]string{"nomad", "rpc", r.endpoint, op}, 1,
		[]metrics.Label{{Name: "id", Value: key}})
	metrics.AddSampleWithLabels(
		[]string{"nomad", "rpc", r.endpoint, op, "used"}, float32(used),
		[]metrics.Label{{Name: "id", Value: key}})

	if err != nil && err != limiter.ErrStopped {
		return err
	}
	if !ok {
		// if we got ErrStopped we'll also send back
		metrics.IncrCounterWithLabels([]string{"nomad", "rpc", r.endpoint, op, "limited"}, 1,
			[]metrics.Label{{Name: "id", Value: key}})
		return structs.ErrTooManyRequests
	}
	return nil
}

func (r *endpointLimiter) close(ctx context.Context) {
	_ = r.write.Close(ctx)
	_ = r.read.Close(ctx)
	_ = r.list.Close(ctx)
}

func newRateLimiterStore(tokens uint64) limiter.Store {
	// note: the memorystore implementation never returns an error
	store, _ := memorystore.New(&memorystore.Config{
		Tokens:        tokens,
		Interval:      time.Minute,
		SweepInterval: time.Hour, // how often to clean up stale entries
		SweepMinTTL:   time.Hour, // how stale entries need to be to clean up
	})
	return store
}
