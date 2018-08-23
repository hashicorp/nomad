package allocrunnerv2

import (
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocwatcher"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/interfaces"
	cstate "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Config holds the configuration for creating an allocation runner.
type Config struct {
	// Logger is the logger for the allocation runner.
	Logger log.Logger

	// ClientConfig is the clients configuration.
	ClientConfig *clientconfig.Config

	// Alloc captures the allocation that should be run.
	Alloc *structs.Allocation

	// StateDB is used to store and restore state.
	StateDB cstate.StateDB

	// Consul is the Consul client used to register task services and checks
	Consul consul.ConsulServiceAPI

	// Vault is the Vault client to use to retrieve Vault tokens
	Vault vaultclient.VaultClient

	// StateUpdater is used to emit updated task state
	StateUpdater interfaces.AllocStateHandler

	// PrevAllocWatcher handles waiting on previous allocations and
	// migrating their ephemeral disk when necessary.
	PrevAllocWatcher allocwatcher.PrevAllocWatcher
}
