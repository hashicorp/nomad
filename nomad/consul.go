package nomad

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/pkg/errors"
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

	// todo: more revocation things
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

type ServiceIdentityIndex struct {
	ClusterID string
	AllocID   string
	TaskName  string
}

func (sii ServiceIdentityIndex) Validate() error {
	switch {
	case sii.ClusterID == "":
		return errors.New("cluster id not set")
	case sii.AllocID == "":
		return errors.New("alloc id not set")
	case sii.TaskName == "":
		return errors.New("task name not set")
	default:
		return nil
	}
}

func (sii ServiceIdentityIndex) Description() string {
	return fmt.Sprintf(siTokenDescriptionFmt, sii.ClusterID, sii.AllocID, sii.TaskName)
}

// ConsulACLsAPI is an abstraction over the consul/api.ACL API used by Nomad
// Server.
type ConsulACLsAPI interface {

	// CheckSIPolicy checks that the given operator token has the equivalent ACL
	// permissiveness that a Service Identity token policy for task would have.
	CheckSIPolicy(ctx context.Context, task, secretID string) error

	// Create instructs Consul to create a Service Identity token.
	CreateToken(context.Context, ServiceIdentityIndex) (*structs.SIToken, error)

	// RevokeTokens instructs Consul to revoke the given token accessors.
	RevokeTokens(context.Context, []*structs.SITokenAccessor) error

	// ListTokens lists every token in Consul.
	//
	// To be used for reconciliation (later).
	ListTokens() ([]string, error)
}

type consulACLsAPI struct {
	// aclClient is the API subset of the real consul client we need for
	// managing Service Identity tokens.
	aclClient consul.ACLsAPI

	// limiter is used to rate limit requests to consul
	limiter *rate.Limiter

	// logger is used to log messages
	logger hclog.Logger
}

func NewConsulACLsAPI(aclClient consul.ACLsAPI, logger hclog.Logger) (ConsulACLsAPI, error) {
	c := &consulACLsAPI{
		aclClient: aclClient,
		logger:    logger.Named("consul_acl"),
		limiter:   rate.NewLimiter(requestRateLimit, int(requestRateLimit)),
	}
	return c, nil
}

func (c *consulACLsAPI) CheckSIPolicy(_ context.Context, task, secretID string) error {
	if id := strings.TrimSpace(secretID); id == "" {
		// todo: check in tests
		return errors.New("missing consul token")
	}

	// todo: log request time, result, etc.

	// todo: use ctx

	// todo: use rate limiting

	opToken, meta, err := c.aclClient.TokenReadSelf(&api.QueryOptions{
		AllowStale: false,
		Token:      secretID,
	})
	if err != nil {
		return errors.Wrap(err, "unable to validate operator consul token")
	}

	_ = meta

	allowable, err := c.hasSufficientPolicy(task, opToken)
	if err != nil {
		return errors.Wrap(err, "unable to validate operator consul token")
	}
	if !allowable {
		return errors.Errorf("permission denied for %q", task)
	}

	return nil
}

func (c *consulACLsAPI) CreateToken(ctx context.Context, sii ServiceIdentityIndex) (*structs.SIToken, error) {
	defer metrics.MeasureSince([]string{"nomad", "consul", "create_token"}, time.Now())

	// sanity check the metadata for the token we want
	if err := sii.Validate(); err != nil {
		return nil, err
	}

	// todo: use ctx

	// todo: rate limiting

	// the token created must be for the service, not the sidecar of the service
	// https://www.consul.io/docs/acl/acl-system.html#acl-service-identities
	serviceName := strings.TrimPrefix(sii.TaskName, structs.ConnectProxyPrefix+"-")
	partial := &api.ACLToken{
		Description:       sii.Description(),
		ServiceIdentities: []*api.ACLServiceIdentity{{ServiceName: serviceName}},
	}

	token, _, err := c.aclClient.TokenCreate(partial, nil)
	if err != nil {
		return nil, err
	}

	return &structs.SIToken{
		TaskName:   sii.TaskName,
		AccessorID: token.AccessorID,
		SecretID:   token.SecretID,
	}, nil
}

func (c *consulACLsAPI) RevokeTokens(ctx context.Context, accessors []*structs.SITokenAccessor) error {
	defer metrics.MeasureSince([]string{"nomad", "consul", "revoke_tokens"}, time.Now())

	// todo: use ctx

	// todo: rate limiting

	for _, accessor := range accessors {
		if err := c.revokeToken(ctx, accessor); err != nil {
			// todo: accumulate errors and IDs that are going to need another attempt
			return err
		}
	}

	return nil
}

func (c *consulACLsAPI) revokeToken(_ context.Context, accessor *structs.SITokenAccessor) error {
	c.logger.Trace("revoke SI token", "task", accessor.TaskName, "alloc_id", accessor.AllocID, "node_id", accessor.NodeID)
	_, err := c.aclClient.TokenDelete(accessor.AccessorID, nil)
	return err
}

func (c *consulACLsAPI) ListTokens() ([]string, error) {
	defer metrics.MeasureSince([]string{"nomad", "consul", "list_tokens"}, time.Now())

	return nil, errors.New("not yet implemented")
}
