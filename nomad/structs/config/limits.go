package config

import (
	"math"

	"github.com/hashicorp/nomad/helper/pointer"
)

const (
	// LimitsNonStreamingConnsPerClient is the number of connections per
	// peer to reserve for non-streaming RPC connections. Since streaming
	// RPCs require their own TCP connection, they have their own limit
	// this amount lower than the overall limit. This reserves a number of
	// connections for Raft and other RPCs.
	//
	// TODO Remove limit once MultiplexV2 is used.
	LimitsNonStreamingConnsPerClient = 20
)

// Limits configures timeout limits similar to Consul's limits configuration
// parameters. Limits is the internal version with the fields parsed.
type Limits struct {
	// HTTPSHandshakeTimeout is the deadline by which HTTPS TLS handshakes
	// must complete.
	//
	// 0 means no timeout.
	HTTPSHandshakeTimeout string `hcl:"https_handshake_timeout"`

	// HTTPMaxConnsPerClient is the maximum number of concurrent HTTP
	// connections from a single IP address. nil/0 means no limit.
	HTTPMaxConnsPerClient *int `hcl:"http_max_conns_per_client"`

	// RPCHandshakeTimeout is the deadline by which RPC handshakes must
	// complete. The RPC handshake includes the first byte read as well as
	// the TLS handshake and subsequent byte read if TLS is enabled.
	//
	// The deadline is reset after the first byte is read so when TLS is
	// enabled RPC connections may take (timeout * 2) to complete.
	//
	// The RPC handshake timeout only applies to servers. 0 means no
	// timeout.
	RPCHandshakeTimeout string `hcl:"rpc_handshake_timeout"`

	// RPCMaxConnsPerClient is the maximum number of concurrent RPC
	// connections from a single IP address. nil/0 means no limit.
	RPCMaxConnsPerClient *int `hcl:"rpc_max_conns_per_client"`

	// RPCDefaultWriteRate is the default maximum write RPC requests
	// per endpoint per user per minute. nil/0 means no limit.
	RPCDefaultWriteRate *int `hcl:"rpc_default_write_rate"`

	// RPCDefaultReadRate is the default maximum read RPC requests per
	// endpoint per user per minute. nil/0 means no limit.
	RPCDefaultReadRate *int `hcl:"rpc_default_read_rate"`

	// RPCDefaultListRate is the default maximum list RPC requests per
	// endpoint per user per minute. nil/0 means no limit.
	RPCDefaultListRate *int `hcl:"rpc_default_list_rate"`

	// Endpoints are the endpoint-specific limits.
	Endpoints *RPCEndpointLimitsSet `hcl:"endpoint_limits"`
}

// RPCEndpointLimitsSet is the set all the RPC rate limits specific to each
// endpoint.
type RPCEndpointLimitsSet struct {
	ACL                 *RPCEndpointLimits `hcl:"acls"`
	Agent               *RPCEndpointLimits `hcl:"client_agent"`
	Alloc               *RPCEndpointLimits `hcl:"allocs"`
	CSIPlugin           *RPCEndpointLimits `hcl:"csi_plugin"`
	CSIVolume           *RPCEndpointLimits `hcl:"csi_volume"`
	ClientAllocations   *RPCEndpointLimits `hcl:"client_allocations"`
	ClientCSI           *RPCEndpointLimits `hcl:"client_csi"`
	ClientStats         *RPCEndpointLimits `hcl:"client_stats"`
	Deployment          *RPCEndpointLimits `hcl:"deployments"`
	Eval                *RPCEndpointLimits `hcl:"evals"`
	Event               *RPCEndpointLimits `hcl:"events"`
	Filesystem          *RPCEndpointLimits `hcl:"client_filesystem"`
	Job                 *RPCEndpointLimits `hcl:"jobs"`
	Keyring             *RPCEndpointLimits `hcl:"keyring"`
	Namespace           *RPCEndpointLimits `hcl:"namespaces"`
	Node                *RPCEndpointLimits `hcl:"nodes"`
	Operator            *RPCEndpointLimits `hcl:"operator"`
	Periodic            *RPCEndpointLimits `hcl:"periodic"`
	Plan                *RPCEndpointLimits `hcl:"plan"`
	Regions             *RPCEndpointLimits `hcl:"regions"`
	Scaling             *RPCEndpointLimits `hcl:"scaling"`
	Search              *RPCEndpointLimits `hcl:"search"`
	SecureVariables     *RPCEndpointLimits `hcl:"secure_variables"`
	ServiceRegistration *RPCEndpointLimits `hcl:"services"`
	Status              *RPCEndpointLimits `hcl:"status"`
	System              *RPCEndpointLimits `hcl:"system"`

	// Enterprise endpoint
	License        *RPCEndpointLimits `hcl:"license"`
	Quota          *RPCEndpointLimits `hcl:"quota"`
	Recommendation *RPCEndpointLimits `hcl:"recommendations"`
	Sentinel       *RPCEndpointLimits `hcl:"sentinel"`
}

// DefaultLimits returns the default limits values. User settings should be
// merged into these defaults.
func DefaultLimits() Limits {
	return Limits{
		HTTPSHandshakeTimeout: "5s",
		HTTPMaxConnsPerClient: pointer.Of(100),
		RPCHandshakeTimeout:   "5s",
		RPCMaxConnsPerClient:  pointer.Of(100),
		RPCDefaultWriteRate:   pointer.Of(math.MaxInt),
		RPCDefaultReadRate:    pointer.Of(math.MaxInt),
		RPCDefaultListRate:    pointer.Of(math.MaxInt),
	}
}

// Merge returns a new Limits where non-empty/nil fields in the argument have
// precedence.
func (l *Limits) Merge(o Limits) Limits {
	m := *l

	if o.HTTPSHandshakeTimeout != "" {
		m.HTTPSHandshakeTimeout = o.HTTPSHandshakeTimeout
	}
	if o.HTTPMaxConnsPerClient != nil {
		m.HTTPMaxConnsPerClient = pointer.Of(*o.HTTPMaxConnsPerClient)
	}
	if o.RPCHandshakeTimeout != "" {
		m.RPCHandshakeTimeout = o.RPCHandshakeTimeout
	}
	if o.RPCMaxConnsPerClient != nil {
		m.RPCMaxConnsPerClient = pointer.Of(*o.RPCMaxConnsPerClient)
	}
	if o.RPCDefaultWriteRate != nil {
		m.RPCDefaultWriteRate = pointer.Of(*o.RPCDefaultWriteRate)
	}
	if o.RPCDefaultReadRate != nil {
		m.RPCDefaultReadRate = pointer.Of(*o.RPCDefaultReadRate)
	}
	if o.RPCDefaultListRate != nil {
		m.RPCDefaultListRate = pointer.Of(*o.RPCDefaultListRate)
	}
	if o.Endpoints != nil {
		o.Endpoints = m.Endpoints.Merge(*o.Endpoints)
	}

	return m
}

// Copy returns a new deep copy of a Limits struct.
func (l *Limits) Copy() Limits {
	c := *l
	if l.HTTPMaxConnsPerClient != nil {
		c.HTTPMaxConnsPerClient = pointer.Of(*l.HTTPMaxConnsPerClient)
	}
	if l.RPCMaxConnsPerClient != nil {
		c.RPCMaxConnsPerClient = pointer.Of(*l.RPCMaxConnsPerClient)
	}
	if l.RPCDefaultWriteRate != nil {
		c.RPCDefaultWriteRate = pointer.Of(*l.RPCDefaultWriteRate)
	}
	if l.RPCDefaultReadRate != nil {
		c.RPCDefaultReadRate = pointer.Of(*l.RPCDefaultReadRate)
	}
	if l.RPCDefaultListRate != nil {
		c.RPCDefaultListRate = pointer.Of(*l.RPCDefaultListRate)
	}
	if l.Endpoints != nil {
		c.Endpoints = l.Endpoints.Copy()
	}

	return c
}

func (l *Limits) Canonicalize() *Limits {
	if l == nil {
		l = &Limits{}
	}
	if l.Endpoints == nil {
		l.Endpoints = &RPCEndpointLimitsSet{}
	}
	return l
}

func (l *RPCEndpointLimitsSet) Merge(o RPCEndpointLimitsSet) *RPCEndpointLimitsSet {
	if l == nil {
		m := o
		return &m
	}
	m := l

	m.ACL = l.ACL.Merge(*o.ACL)
	m.Agent = l.Agent.Merge(*o.Agent)
	m.Alloc = l.Alloc.Merge(*o.Alloc)
	m.CSIPlugin = l.CSIPlugin.Merge(*o.CSIPlugin)
	m.CSIVolume = l.CSIVolume.Merge(*o.CSIVolume)
	m.ClientAllocations = l.ClientAllocations.Merge(*o.ClientAllocations)
	m.ClientCSI = l.ClientCSI.Merge(*o.ClientCSI)
	m.ClientStats = l.ClientStats.Merge(*o.ClientStats)
	m.Deployment = l.Deployment.Merge(*o.Deployment)
	m.Eval = l.Eval.Merge(*o.Eval)
	m.Event = l.Event.Merge(*o.Event)
	m.Filesystem = l.Filesystem.Merge(*o.Filesystem)
	m.Job = l.Job.Merge(*o.Job)
	m.Keyring = l.Keyring.Merge(*o.Keyring)
	m.Namespace = l.Namespace.Merge(*o.Namespace)
	m.Node = l.Node.Merge(*o.Node)
	m.Operator = l.Operator.Merge(*o.Operator)
	m.Periodic = l.Periodic.Merge(*o.Periodic)
	m.Plan = l.Plan.Merge(*o.Plan)
	m.Regions = l.Regions.Merge(*o.Regions)
	m.Scaling = l.Scaling.Merge(*o.Scaling)
	m.Search = l.Search.Merge(*o.Search)
	m.SecureVariables = l.SecureVariables.Merge(*o.SecureVariables)
	m.ServiceRegistration = l.ServiceRegistration.Merge(*o.ServiceRegistration)
	m.Status = l.Status.Merge(*o.Status)
	m.System = l.System.Merge(*o.System)

	// Enterprise config
	m.License = l.License.Merge(*o.License)
	m.Quota = l.Quota.Merge(*o.Quota)
	m.Recommendation = l.Recommendation.Merge(*o.Recommendation)
	m.Sentinel = l.Sentinel.Merge(*o.Sentinel)

	return m
}

func (l *RPCEndpointLimitsSet) Copy() *RPCEndpointLimitsSet {
	c := *l

	c.ACL = l.ACL.Copy()
	c.Agent = l.Agent.Copy()
	c.Alloc = l.Alloc.Copy()
	c.CSIPlugin = l.CSIPlugin.Copy()
	c.CSIVolume = l.CSIVolume.Copy()
	c.ClientAllocations = l.ClientAllocations.Copy()
	c.ClientCSI = l.ClientCSI.Copy()
	c.ClientStats = l.ClientStats.Copy()
	c.Deployment = l.Deployment.Copy()
	c.Eval = l.Eval.Copy()
	c.Event = l.Event.Copy()
	c.Filesystem = l.Filesystem.Copy()
	c.Job = l.Job.Copy()
	c.Keyring = l.Keyring.Copy()
	c.Namespace = l.Namespace.Copy()
	c.Node = l.Node.Copy()
	c.Operator = l.Operator.Copy()
	c.Periodic = l.Periodic.Copy()
	c.Plan = l.Plan.Copy()
	c.Regions = l.Regions.Copy()
	c.Scaling = l.Scaling.Copy()
	c.Search = l.Search.Copy()
	c.SecureVariables = l.SecureVariables.Copy()
	c.ServiceRegistration = l.ServiceRegistration.Copy()
	c.Status = l.Status.Copy()
	c.System = l.System.Copy()

	// Enterprise config
	c.License = l.License.Copy()
	c.Quota = l.Quota.Copy()
	c.Recommendation = l.Recommendation.Copy()
	c.Sentinel = l.Sentinel.Copy()

	return &c
}

type RPCEndpointLimits struct {
	RPCWriteRate *int `hcl:"write_rate"`
	RPCReadRate  *int `hcl:"read_rate"`
	RPCListRate  *int `hcl:"list_rate"`
}

func (l *RPCEndpointLimits) Merge(o RPCEndpointLimits) *RPCEndpointLimits {
	if l == nil {
		m := o
		return &m
	}
	m := l
	if o.RPCWriteRate != nil {
		m.RPCWriteRate = pointer.Of(*o.RPCWriteRate)
	}
	if o.RPCReadRate != nil {
		m.RPCReadRate = pointer.Of(*o.RPCReadRate)
	}
	if o.RPCListRate != nil {
		m.RPCListRate = pointer.Of(*o.RPCListRate)
	}
	return m
}

func (l *RPCEndpointLimits) Copy() *RPCEndpointLimits {
	if l == nil {
		return nil
	}
	c := l
	if l.RPCWriteRate != nil {
		c.RPCWriteRate = pointer.Of(*l.RPCWriteRate)
	}
	if l.RPCReadRate != nil {
		c.RPCReadRate = pointer.Of(*l.RPCReadRate)
	}
	if l.RPCListRate != nil {
		c.RPCListRate = pointer.Of(*l.RPCListRate)
	}
	return c
}
