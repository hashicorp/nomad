package nomad

import (
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

const (
	DefaultRegion   = "global"
	DefaultDC       = "dc1"
	DefaultSerfPort = 4648
)

// These are the protocol versions that Nomad can understand
const (
	ProtocolVersionMin uint8 = 1
	ProtocolVersionMax       = 1
)

// ProtocolVersionMap is the mapping of Nomad protocol versions
// to Serf protocol versions. We mask the Serf protocols using
// our own protocol version.
var protocolVersionMap map[uint8]uint8

func init() {
	protocolVersionMap = map[uint8]uint8{
		1: 4,
	}
}

var (
	DefaultRPCAddr = &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4647}
)

// Config is used to parameterize the server
type Config struct {
	// Bootstrap mode is used to bring up the first Nomad server.  It is
	// required so that it can elect a leader without any other nodes
	// being present
	Bootstrap bool

	// BootstrapExpect mode is used to automatically bring up a
	// collection of Nomad servers. This can be used to automatically
	// bring up a collection of nodes.  All operations on BootstrapExpect
	// must be handled via `atomic.*Int32()` calls.
	BootstrapExpect int32

	// DataDir is the directory to store our state in
	DataDir string

	// DevMode is used for development purposes only and limits the
	// use of persistence or state.
	DevMode bool

	// DevDisableBootstrap is used to disable bootstrap mode while
	// in DevMode. This is largely used for testing.
	DevDisableBootstrap bool

	// LogOutput is the location to write logs to. If this is not set,
	// logs will go to stderr.
	LogOutput io.Writer

	// ProtocolVersion is the protocol version to speak. This must be between
	// ProtocolVersionMin and ProtocolVersionMax.
	ProtocolVersion uint8

	// RPCAddr is the RPC address used by Nomad. This should be reachable
	// by the other servers and clients
	RPCAddr *net.TCPAddr

	// RPCAdvertise is the address that is advertised to other nodes for
	// the RPC endpoint. This can differ from the RPC address, if for example
	// the RPCAddr is unspecified "0.0.0.0:4646", but this address must be
	// reachable
	RPCAdvertise *net.TCPAddr

	// RaftConfig is the configuration used for Raft in the local DC
	RaftConfig *raft.Config

	// RaftTimeout is applied to any network traffic for raft. Defaults to 10s.
	RaftTimeout time.Duration

	// SerfConfig is the configuration for the serf cluster
	SerfConfig *serf.Config

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string

	// Region is the region this Nomad server belongs to.
	Region string

	// AuthoritativeRegion is the region which is treated as the authoritative source
	// for ACLs and Policies. This provides a single source of truth to resolve conflicts.
	AuthoritativeRegion string

	// Datacenter is the datacenter this Nomad server belongs to.
	Datacenter string

	// Build is a string that is gossiped around, and can be used to help
	// operators track which versions are actively deployed
	Build string

	// NumSchedulers is the number of scheduler thread that are run.
	// This can be as many as one per core, or zero to disable this server
	// from doing any scheduling work.
	NumSchedulers int

	// EnabledSchedulers controls the set of sub-schedulers that are
	// enabled for this server to handle. This will restrict the evaluations
	// that the workers dequeue for processing.
	EnabledSchedulers []string

	// ReconcileInterval controls how often we reconcile the strongly
	// consistent store with the Serf info. This is used to handle nodes
	// that are force removed, as well as intermittent unavailability during
	// leader election.
	ReconcileInterval time.Duration

	// EvalGCInterval is how often we dispatch a job to GC evaluations
	EvalGCInterval time.Duration

	// EvalGCThreshold is how "old" an evaluation must be to be eligible
	// for GC. This gives users some time to debug a failed evaluation.
	EvalGCThreshold time.Duration

	// JobGCInterval is how often we dispatch a job to GC jobs that are
	// available for garbage collection.
	JobGCInterval time.Duration

	// JobGCThreshold is how old a job must be before it eligible for GC. This gives
	// the user time to inspect the job.
	JobGCThreshold time.Duration

	// NodeGCInterval is how often we dispatch a job to GC failed nodes.
	NodeGCInterval time.Duration

	// NodeGCThreshold is how "old" a node must be to be eligible
	// for GC. This gives users some time to view and debug a failed nodes.
	NodeGCThreshold time.Duration

	// DeploymentGCInterval is how often we dispatch a job to GC terminal
	// deployments.
	DeploymentGCInterval time.Duration

	// DeploymentGCThreshold is how "old" a deployment must be to be eligible
	// for GC. This gives users some time to view terminal deployments.
	DeploymentGCThreshold time.Duration

	// EvalNackTimeout controls how long we allow a sub-scheduler to
	// work on an evaluation before we consider it failed and Nack it.
	// This allows that evaluation to be handed to another sub-scheduler
	// to work on. Defaults to 60 seconds. This should be long enough that
	// no evaluation hits it unless the sub-scheduler has failed.
	EvalNackTimeout time.Duration

	// EvalDeliveryLimit is the limit of attempts we make to deliver and
	// process an evaluation. This is used so that an eval that will never
	// complete eventually fails out of the system.
	EvalDeliveryLimit int

	// EvalNackInitialReenqueueDelay is the delay applied before reenqueuing a
	// Nacked evaluation for the first time. This value should be small as the
	// initial Nack can be due to a down machine and the eval should be retried
	// quickly for liveliness.
	EvalNackInitialReenqueueDelay time.Duration

	// EvalNackSubsequentReenqueueDelay is the delay applied before reenqueuing
	// an evaluation that has been Nacked more than once. This delay is
	// compounding after the first Nack. This value should be significantly
	// longer than the initial delay as the purpose it severs is to apply
	// back-pressure as evaluatiions are being Nacked either due to scheduler
	// failures or because they are hitting their Nack timeout, both of which
	// are signs of high server resource usage.
	EvalNackSubsequentReenqueueDelay time.Duration

	// EvalFailedFollowupBaselineDelay is the minimum time waited before
	// retrying a failed evaluation.
	EvalFailedFollowupBaselineDelay time.Duration

	// EvalFailedFollowupDelayRange defines the range of additional time from
	// the baseline in which to wait before retrying a failed evaluation. The
	// additional delay is selected from this range randomly.
	EvalFailedFollowupDelayRange time.Duration

	// MinHeartbeatTTL is the minimum time between heartbeats.
	// This is used as a floor to prevent excessive updates.
	MinHeartbeatTTL time.Duration

	// MaxHeartbeatsPerSecond is the maximum target rate of heartbeats
	// being processed per second. This allows the TTL to be increased
	// to meet the target rate.
	MaxHeartbeatsPerSecond float64

	// HeartbeatGrace is the additional time given as a grace period
	// beyond the TTL to account for network and processing delays
	// as well as clock skew.
	HeartbeatGrace time.Duration

	// FailoverHeartbeatTTL is the TTL applied to heartbeats after
	// a new leader is elected, since we no longer know the status
	// of all the heartbeats.
	FailoverHeartbeatTTL time.Duration

	// ConsulConfig is this Agent's Consul configuration
	ConsulConfig *config.ConsulConfig

	// VaultConfig is this Agent's Vault configuration
	VaultConfig *config.VaultConfig

	// RPCHoldTimeout is how long an RPC can be "held" before it is errored.
	// This is used to paper over a loss of leadership by instead holding RPCs,
	// so that the caller experiences a slow response rather than an error.
	// This period is meant to be long enough for a leader election to take
	// place, and a small jitter is applied to avoid a thundering herd.
	RPCHoldTimeout time.Duration

	// TLSConfig holds various TLS related configurations
	TLSConfig *config.TLSConfig

	// ACLEnabled controls if ACL enforcement and management is enabled.
	ACLEnabled bool

	// ReplicationBackoff is how much we backoff when replication errors.
	// This is a tunable knob for testing primarily.
	ReplicationBackoff time.Duration

	// ReplicationToken is the ACL Token Secret ID used to fetch from
	// the Authoritative Region.
	ReplicationToken string

	// SentinelGCInterval is the interval that we GC unused policies.
	SentinelGCInterval time.Duration

	// SentinelConfig is this Agent's Sentinel configuration
	SentinelConfig *config.SentinelConfig

	// StatsCollectionInterval is the interval at which the Nomad server
	// publishes metrics which are periodic in nature like updating gauges
	StatsCollectionInterval time.Duration

	// DisableTaggedMetrics determines whether metrics will be displayed via a
	// key/value/tag format, or simply a key/value format
	DisableTaggedMetrics bool

	// BackwardsCompatibleMetrics determines whether to show methods of
	// displaying metrics for older verions, or to only show the new format
	BackwardsCompatibleMetrics bool
}

// CheckVersion is used to check if the ProtocolVersion is valid
func (c *Config) CheckVersion() error {
	if c.ProtocolVersion < ProtocolVersionMin {
		return fmt.Errorf("Protocol version '%d' too low. Must be in range: [%d, %d]",
			c.ProtocolVersion, ProtocolVersionMin, ProtocolVersionMax)
	} else if c.ProtocolVersion > ProtocolVersionMax {
		return fmt.Errorf("Protocol version '%d' too high. Must be in range: [%d, %d]",
			c.ProtocolVersion, ProtocolVersionMin, ProtocolVersionMax)
	}
	return nil
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	c := &Config{
		Region:                           DefaultRegion,
		AuthoritativeRegion:              DefaultRegion,
		Datacenter:                       DefaultDC,
		NodeName:                         hostname,
		ProtocolVersion:                  ProtocolVersionMax,
		RaftConfig:                       raft.DefaultConfig(),
		RaftTimeout:                      10 * time.Second,
		LogOutput:                        os.Stderr,
		RPCAddr:                          DefaultRPCAddr,
		SerfConfig:                       serf.DefaultConfig(),
		NumSchedulers:                    1,
		ReconcileInterval:                60 * time.Second,
		EvalGCInterval:                   5 * time.Minute,
		EvalGCThreshold:                  1 * time.Hour,
		JobGCInterval:                    5 * time.Minute,
		JobGCThreshold:                   4 * time.Hour,
		NodeGCInterval:                   5 * time.Minute,
		NodeGCThreshold:                  24 * time.Hour,
		DeploymentGCInterval:             5 * time.Minute,
		DeploymentGCThreshold:            1 * time.Hour,
		EvalNackTimeout:                  60 * time.Second,
		EvalDeliveryLimit:                3,
		EvalNackInitialReenqueueDelay:    1 * time.Second,
		EvalNackSubsequentReenqueueDelay: 20 * time.Second,
		EvalFailedFollowupBaselineDelay:  1 * time.Minute,
		EvalFailedFollowupDelayRange:     5 * time.Minute,
		MinHeartbeatTTL:                  10 * time.Second,
		MaxHeartbeatsPerSecond:           50.0,
		HeartbeatGrace:                   10 * time.Second,
		FailoverHeartbeatTTL:             300 * time.Second,
		ConsulConfig:                     config.DefaultConsulConfig(),
		VaultConfig:                      config.DefaultVaultConfig(),
		RPCHoldTimeout:                   5 * time.Second,
		TLSConfig:                        &config.TLSConfig{},
		ReplicationBackoff:               30 * time.Second,
		SentinelGCInterval:               30 * time.Second,
		StatsCollectionInterval:          1 * time.Minute,
	}

	// Enable all known schedulers by default
	c.EnabledSchedulers = make([]string, 0, len(scheduler.BuiltinSchedulers))
	for name := range scheduler.BuiltinSchedulers {
		c.EnabledSchedulers = append(c.EnabledSchedulers, name)
	}
	c.EnabledSchedulers = append(c.EnabledSchedulers, structs.JobTypeCore)

	// Default the number of schedulers to match the coores
	c.NumSchedulers = runtime.NumCPU()

	// Increase our reap interval to 3 days instead of 24h.
	c.SerfConfig.ReconnectTimeout = 3 * 24 * time.Hour

	// Serf should use the WAN timing, since we are using it
	// to communicate between DC's
	c.SerfConfig.MemberlistConfig = memberlist.DefaultWANConfig()
	c.SerfConfig.MemberlistConfig.BindPort = DefaultSerfPort

	// Disable shutdown on removal
	c.RaftConfig.ShutdownOnRemove = false

	// Enable interoperability with unversioned Raft library, and don't
	// start using new ID-based features yet.
	c.RaftConfig.ProtocolVersion = 1

	return c
}

// tlsConfig returns a TLSUtil Config based on the server configuration
func (c *Config) tlsConfig() *tlsutil.Config {
	tlsConf := &tlsutil.Config{
		VerifyIncoming:       true,
		VerifyOutgoing:       true,
		VerifyServerHostname: c.TLSConfig.VerifyServerHostname,
		CAFile:               c.TLSConfig.CAFile,
		CertFile:             c.TLSConfig.CertFile,
		KeyFile:              c.TLSConfig.KeyFile,
	}
	return tlsConf
}
