// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	arstate "github.com/hashicorp/nomad/client/allocrunner/state"
	trstate "github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	driverstate "github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/boltdd"
	"github.com/hashicorp/nomad/nomad/structs"
	"go.etcd.io/bbolt"
)

/*
The client has a boltDB backed state store. The schema as of 0.9 looks as follows:

meta/
|--> version -> '2' (not msgpack encoded)
|--> upgraded -> time.Now().Format(timeRFC3339)
allocations/
|--> <alloc-id>/
   |--> alloc          -> allocEntry{*structs.Allocation}
	 |--> deploy_status  -> deployStatusEntry{*structs.AllocDeploymentStatus}
	 |--> network_status -> networkStatusEntry{*structs.AllocNetworkStatus}
	 |--> acknowledged_state -> acknowledgedStateEntry{*arstate.State}
	 |--> alloc_volumes -> allocVolumeStatesEntry{arstate.AllocVolumes}
   |--> task-<name>/
      |--> local_state -> *trstate.LocalState # Local-only state
      |--> task_state  -> *structs.TaskState  # Syncs to servers
   |--> checks/
      |--> check-<id> -> *structs.CheckState # Syncs to servers

devicemanager/
|--> plugin_state -> *dmstate.PluginState

drivermanager/
|--> plugin_state -> *driverstate.PluginState

dynamicplugins/
|--> registry_state -> *dynamicplugins.RegistryState

nodemeta/
|--> meta -> map[string]*string

node/
|--> registration -> *cstructs.NodeRegistration
*/

var (
	// metaBucketName is the name of the metadata bucket
	metaBucketName = []byte("meta")

	// metaVersionKey is the key the state schema version is stored under.
	metaVersionKey = []byte("version")

	// metaVersion is the value of the state schema version to detect when
	// an upgrade is needed. It skips the usual boltdd/msgpack backend to
	// be as portable and futureproof as possible.
	metaVersion = []byte{'3'}

	// metaUpgradedKey is the key that stores the timestamp of the last
	// time the schema was upgraded.
	metaUpgradedKey = []byte("upgraded")

	// allocationsBucketName is the bucket name containing all allocation related
	// data
	allocationsBucketName = []byte("allocations")

	// allocKey is the key Allocations are stored under encapsulated in
	// allocEntry structs.
	allocKey = []byte("alloc")

	// allocDeployStatusKey is the key *structs.AllocDeploymentStatus is
	// stored under.
	allocDeployStatusKey = []byte("deploy_status")

	// allocNetworkStatusKey is the key *structs.AllocNetworkStatus is
	// stored under
	allocNetworkStatusKey = []byte("network_status")

	// acknowledgedStateKey is the key *arstate.State is stored under
	acknowledgedStateKey = []byte("acknowledged_state")

	allocVolumeKey = []byte("alloc_volume")

	// checkResultsBucket is the bucket name in which check query results are stored
	checkResultsBucket = []byte("check_results")

	// allocations -> $allocid -> task-$taskname -> the keys below
	taskLocalStateKey = []byte("local_state")
	taskStateKey      = []byte("task_state")

	// devManagerBucket is the bucket name containing all device manager related
	// data
	devManagerBucket = []byte("devicemanager")

	// driverManagerBucket is the bucket name containing all driver manager
	// related data
	driverManagerBucket = []byte("drivermanager")

	// managerPluginStateKey is the key by which plugin manager plugin state is
	// stored at
	managerPluginStateKey = []byte("plugin_state")

	// dynamicPluginBucketName is the bucket name containing all dynamic plugin
	// registry data. each dynamic plugin registry will have its own subbucket.
	dynamicPluginBucketName = []byte("dynamicplugins")

	// registryStateKey is the key at which dynamic plugin registry state is stored
	registryStateKey = []byte("registry_state")

	// nodeMetaBucket is the bucket name in which dynamically updated node
	// metadata is stored
	nodeMetaBucket = []byte("nodemeta")

	// nodeMetaKey is the key at which dynamic node metadata is stored.
	nodeMetaKey = []byte("meta")

	// nodeBucket is the bucket name in which data about the node is stored.
	nodeBucket = []byte("node")

	// nodeRegistrationKey is the key at which node registration data is stored.
	nodeRegistrationKey = []byte("node_registration")
)

// taskBucketName returns the bucket name for the given task name.
func taskBucketName(taskName string) []byte {
	return []byte("task-" + taskName)
}

// NewStateDBFunc creates a StateDB given a state directory.
type NewStateDBFunc func(logger hclog.Logger, stateDir string) (StateDB, error)

// GetStateDBFactory returns a func for creating a StateDB
func GetStateDBFactory(devMode bool) NewStateDBFunc {
	// Return a noop state db implementation when in debug mode
	if devMode {
		return func(hclog.Logger, string) (StateDB, error) {
			return NoopDB{}, nil
		}
	}

	return NewBoltStateDB
}

// BoltStateDB persists and restores Nomad client state in a boltdb. All
// methods are safe for concurrent access.
type BoltStateDB struct {
	stateDir string
	db       *boltdd.DB
	logger   hclog.Logger
}

// NewBoltStateDB creates or opens an existing boltdb state file or returns an
// error.
func NewBoltStateDB(logger hclog.Logger, stateDir string) (StateDB, error) {
	fn := filepath.Join(stateDir, "state.db")

	// Check to see if the DB already exists
	fi, err := os.Stat(fn)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	firstRun := fi == nil

	// Timeout to force failure when accessing a data dir that is already in use
	timeout := &bbolt.Options{Timeout: 5 * time.Second}

	// Create or open the boltdb state database
	db, err := boltdd.Open(fn, 0600, timeout)
	if err == bbolt.ErrTimeout {
		return nil, fmt.Errorf("timed out while opening database, is another Nomad process accessing data_dir %s?", stateDir)
	} else if err != nil {
		return nil, fmt.Errorf("failed to create state database: %v", err)
	}

	sdb := &BoltStateDB{
		stateDir: stateDir,
		db:       db,
		logger:   logger,
	}

	// If db did not already exist, initialize metadata fields
	if firstRun {
		if err := sdb.init(); err != nil {
			return nil, err
		}
	}

	return sdb, nil
}

func (s *BoltStateDB) Name() string {
	return "boltdb"
}

// GetAllAllocations gets all allocations persisted by this client and returns
// a map of alloc ids to errors for any allocations that could not be restored.
//
// If a fatal error was encountered it will be returned and the other two
// values will be nil.
func (s *BoltStateDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	var allocs []*structs.Allocation
	var errs map[string]error

	err := s.db.View(func(tx *boltdd.Tx) error {
		allocs, errs = s.getAllAllocations(tx)
		return nil
	})

	// db.View itself may return an error, so still check
	if err != nil {
		return nil, nil, err
	}

	return allocs, errs, nil
}

// allocEntry wraps values in the Allocations buckets
type allocEntry struct {
	Alloc *structs.Allocation
}

func (s *BoltStateDB) getAllAllocations(tx *boltdd.Tx) ([]*structs.Allocation, map[string]error) {
	allocs := make([]*structs.Allocation, 0, 10)
	errs := map[string]error{}

	allocationsBkt := tx.Bucket(allocationsBucketName)
	if allocationsBkt == nil {
		// No allocs
		return allocs, errs
	}

	// Create a cursor for iteration.
	c := allocationsBkt.BoltBucket().Cursor()

	// Iterate over all the allocation buckets
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		allocID := string(k)
		allocBkt := allocationsBkt.Bucket(k)
		if allocBkt == nil {
			errs[allocID] = fmt.Errorf("missing alloc bucket")
			continue
		}

		var ae allocEntry
		if err := allocBkt.Get(allocKey, &ae); err != nil {
			errs[allocID] = fmt.Errorf("failed to decode alloc: %v", err)
			continue
		}

		// Handle upgrade path
		ae.Alloc.Canonicalize()
		ae.Alloc.Job.Canonicalize()

		allocs = append(allocs, ae.Alloc)
	}

	return allocs, errs
}

// PutAllocation stores an allocation or returns an error.
func (s *BoltStateDB) PutAllocation(alloc *structs.Allocation, opts ...WriteOption) error {
	return s.updateWithOptions(opts, func(tx *boltdd.Tx) error {
		// Retrieve the root allocations bucket
		allocsBkt, err := tx.CreateBucketIfNotExists(allocationsBucketName)
		if err != nil {
			return err
		}

		// Retrieve the specific allocations bucket
		key := []byte(alloc.ID)
		allocBkt, err := allocsBkt.CreateBucketIfNotExists(key)
		if err != nil {
			return err
		}

		allocState := allocEntry{
			Alloc: alloc,
		}
		return allocBkt.Put(allocKey, &allocState)
	})
}

// deployStatusEntry wraps values for DeploymentStatus keys.
type deployStatusEntry struct {
	DeploymentStatus *structs.AllocDeploymentStatus
}

// PutDeploymentStatus stores an allocation's DeploymentStatus or returns an
// error.
func (s *BoltStateDB) PutDeploymentStatus(allocID string, ds *structs.AllocDeploymentStatus) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		return putDeploymentStatusImpl(tx, allocID, ds)
	})
}

func putDeploymentStatusImpl(tx *boltdd.Tx, allocID string, ds *structs.AllocDeploymentStatus) error {
	allocBkt, err := getAllocationBucket(tx, allocID)
	if err != nil {
		return err
	}

	entry := deployStatusEntry{
		DeploymentStatus: ds,
	}
	return allocBkt.Put(allocDeployStatusKey, &entry)
}

// GetDeploymentStatus retrieves an allocation's DeploymentStatus or returns an
// error.
func (s *BoltStateDB) GetDeploymentStatus(allocID string) (*structs.AllocDeploymentStatus, error) {
	var entry deployStatusEntry

	err := s.db.View(func(tx *boltdd.Tx) error {
		allAllocsBkt := tx.Bucket(allocationsBucketName)
		if allAllocsBkt == nil {
			// No state, return
			return nil
		}

		allocBkt := allAllocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			// No state for alloc, return
			return nil
		}

		return allocBkt.Get(allocDeployStatusKey, &entry)
	})

	// It's valid for this field to be nil/missing
	if boltdd.IsErrNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return entry.DeploymentStatus, nil
}

// networkStatusEntry wraps values for NetworkStatus keys.
type networkStatusEntry struct {
	NetworkStatus *structs.AllocNetworkStatus
}

// PutNetworkStatus stores an allocation's DeploymentStatus or returns an
// error.
func (s *BoltStateDB) PutNetworkStatus(allocID string, ds *structs.AllocNetworkStatus, opts ...WriteOption) error {
	return s.updateWithOptions(opts, func(tx *boltdd.Tx) error {
		return putNetworkStatusImpl(tx, allocID, ds)
	})
}

func putNetworkStatusImpl(tx *boltdd.Tx, allocID string, ds *structs.AllocNetworkStatus) error {
	allocBkt, err := getAllocationBucket(tx, allocID)
	if err != nil {
		return err
	}

	entry := networkStatusEntry{
		NetworkStatus: ds,
	}
	return allocBkt.Put(allocNetworkStatusKey, &entry)
}

// GetNetworkStatus retrieves an allocation's NetworkStatus or returns an
// error.
func (s *BoltStateDB) GetNetworkStatus(allocID string) (*structs.AllocNetworkStatus, error) {
	var entry networkStatusEntry

	err := s.db.View(func(tx *boltdd.Tx) error {
		allAllocsBkt := tx.Bucket(allocationsBucketName)
		if allAllocsBkt == nil {
			// No state, return
			return nil
		}

		allocBkt := allAllocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			// No state for alloc, return
			return nil
		}

		return allocBkt.Get(allocNetworkStatusKey, &entry)
	})

	// It's valid for this field to be nil/missing
	if boltdd.IsErrNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return entry.NetworkStatus, nil
}

// PutAcknowledgedState stores an allocation's last acknowledged state or
// returns an error if it could not be stored.
func (s *BoltStateDB) PutAcknowledgedState(allocID string, state *arstate.State, opts ...WriteOption) error {
	return s.updateWithOptions(opts, func(tx *boltdd.Tx) error {
		allocBkt, err := getAllocationBucket(tx, allocID)
		if err != nil {
			return err
		}

		entry := acknowledgedStateEntry{
			State: state,
		}
		return allocBkt.Put(acknowledgedStateKey, &entry)
	})
}

// GetAcknowledgedState retrieves an allocation's last acknowledged state
func (s *BoltStateDB) GetAcknowledgedState(allocID string) (*arstate.State, error) {
	var entry acknowledgedStateEntry

	err := s.db.View(func(tx *boltdd.Tx) error {
		allAllocsBkt := tx.Bucket(allocationsBucketName)
		if allAllocsBkt == nil {
			// No state, return
			return nil
		}

		allocBkt := allAllocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			// No state for alloc, return
			return nil
		}

		return allocBkt.Get(acknowledgedStateKey, &entry)
	})

	// It's valid for this field to be nil/missing
	if boltdd.IsErrNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return entry.State, nil
}

type allocVolumeStatesEntry struct {
	State *arstate.AllocVolumes
}

// PutAllocVolumes stores stubs of an allocation's dynamic volume mounts so they
// can be restored.
func (s *BoltStateDB) PutAllocVolumes(allocID string, state *arstate.AllocVolumes, opts ...WriteOption) error {
	return s.updateWithOptions(opts, func(tx *boltdd.Tx) error {
		allocBkt, err := getAllocationBucket(tx, allocID)
		if err != nil {
			return err
		}

		entry := allocVolumeStatesEntry{
			State: state,
		}
		return allocBkt.Put(allocVolumeKey, &entry)
	})
}

// GetAllocVolumes retrieves stubs of an allocation's dynamic volume mounts so
// they can be restored.
func (s *BoltStateDB) GetAllocVolumes(allocID string) (*arstate.AllocVolumes, error) {
	var entry allocVolumeStatesEntry

	err := s.db.View(func(tx *boltdd.Tx) error {
		allAllocsBkt := tx.Bucket(allocationsBucketName)
		if allAllocsBkt == nil {
			// No state, return
			return nil
		}

		allocBkt := allAllocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			// No state for alloc, return
			return nil
		}

		return allocBkt.Get(allocVolumeKey, &entry)
	})

	// It's valid for this field to be nil/missing
	if boltdd.IsErrNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return entry.State, nil
}

// GetTaskRunnerState returns the LocalState and TaskState for a
// TaskRunner. LocalState or TaskState will be nil if they do not exist.
//
// If an error is encountered both LocalState and TaskState will be nil.
func (s *BoltStateDB) GetTaskRunnerState(allocID, taskName string) (*trstate.LocalState, *structs.TaskState, error) {
	var ls *trstate.LocalState
	var ts *structs.TaskState

	err := s.db.View(func(tx *boltdd.Tx) error {
		allAllocsBkt := tx.Bucket(allocationsBucketName)
		if allAllocsBkt == nil {
			// No state, return
			return nil
		}

		allocBkt := allAllocsBkt.Bucket([]byte(allocID))
		if allocBkt == nil {
			// No state for alloc, return
			return nil
		}

		taskBkt := allocBkt.Bucket(taskBucketName(taskName))
		if taskBkt == nil {
			// No state for task, return
			return nil
		}

		// Restore Local State if it exists
		ls = &trstate.LocalState{}
		if err := taskBkt.Get(taskLocalStateKey, ls); err != nil {
			if !boltdd.IsErrNotFound(err) {
				return fmt.Errorf("failed to read local task runner state: %v", err)
			}

			// Key not found, reset ls to nil
			ls = nil
		}

		// Restore Task State if it exists
		ts = &structs.TaskState{}
		if err := taskBkt.Get(taskStateKey, ts); err != nil {
			if !boltdd.IsErrNotFound(err) {
				return fmt.Errorf("failed to read task state: %v", err)
			}

			// Key not found, reset ts to nil
			ts = nil
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return ls, ts, nil
}

// PutTaskRunnerLocalState stores TaskRunner's LocalState or returns an error.
func (s *BoltStateDB) PutTaskRunnerLocalState(allocID, taskName string, val *trstate.LocalState) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		return putTaskRunnerLocalStateImpl(tx, allocID, taskName, val)
	})
}

// putTaskRunnerLocalStateImpl stores TaskRunner's LocalState in an ongoing
// transaction or returns an error.
func putTaskRunnerLocalStateImpl(tx *boltdd.Tx, allocID, taskName string, val *trstate.LocalState) error {
	taskBkt, err := getTaskBucket(tx, allocID, taskName)
	if err != nil {
		return fmt.Errorf("failed to retrieve allocation bucket: %v", err)
	}

	if err := taskBkt.Put(taskLocalStateKey, val); err != nil {
		return fmt.Errorf("failed to write task_runner state: %v", err)
	}

	return nil
}

// PutTaskState stores a task's state or returns an error.
func (s *BoltStateDB) PutTaskState(allocID, taskName string, state *structs.TaskState) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		return putTaskStateImpl(tx, allocID, taskName, state)
	})
}

// putTaskStateImpl stores a task's state in an ongoing transaction or returns
// an error.
func putTaskStateImpl(tx *boltdd.Tx, allocID, taskName string, state *structs.TaskState) error {
	taskBkt, err := getTaskBucket(tx, allocID, taskName)
	if err != nil {
		return fmt.Errorf("failed to retrieve allocation bucket: %v", err)
	}

	return taskBkt.Put(taskStateKey, state)
}

// DeleteTaskBucket is used to delete a task bucket if it exists.
func (s *BoltStateDB) DeleteTaskBucket(allocID, taskName string) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		// Retrieve the root allocations bucket
		allocations := tx.Bucket(allocationsBucketName)
		if allocations == nil {
			return nil
		}

		// Retrieve the specific allocations bucket
		alloc := allocations.Bucket([]byte(allocID))
		if alloc == nil {
			return nil
		}

		// Check if the bucket exists
		key := taskBucketName(taskName)
		return alloc.DeleteBucket(key)
	})
}

// DeleteAllocationBucket is used to delete an allocation bucket if it exists.
func (s *BoltStateDB) DeleteAllocationBucket(allocID string, opts ...WriteOption) error {
	return s.updateWithOptions(opts, func(tx *boltdd.Tx) error {
		// Retrieve the root allocations bucket
		allocations := tx.Bucket(allocationsBucketName)
		if allocations == nil {
			return nil
		}

		key := []byte(allocID)
		return allocations.DeleteBucket(key)
	})
}

// Close releases all database resources and unlocks the database file on disk.
// All transactions must be closed before closing the database.
func (s *BoltStateDB) Close() error {
	return s.db.Close()
}

// getAllocationBucket returns the bucket used to persist state about a
// particular allocation. If the root allocation bucket or the specific
// allocation bucket doesn't exist, it will be created as long as the
// transaction is writable.
func getAllocationBucket(tx *boltdd.Tx, allocID string) (*boltdd.Bucket, error) {
	var err error
	w := tx.Writable()

	// Retrieve the root allocations bucket
	allocations := tx.Bucket(allocationsBucketName)
	if allocations == nil {
		if !w {
			return nil, fmt.Errorf("Allocations bucket doesn't exist and transaction is not writable")
		}

		allocations, err = tx.CreateBucketIfNotExists(allocationsBucketName)
		if err != nil {
			return nil, err
		}
	}

	// Retrieve the specific allocations bucket
	key := []byte(allocID)
	alloc := allocations.Bucket(key)
	if alloc == nil {
		if !w {
			return nil, fmt.Errorf("Allocation bucket doesn't exist and transaction is not writable")
		}

		alloc, err = allocations.CreateBucket(key)
		if err != nil {
			return nil, err
		}
	}

	return alloc, nil
}

// getTaskBucket returns the bucket used to persist state about a
// particular task. If the root allocation bucket, the specific
// allocation or task bucket doesn't exist, they will be created as long as the
// transaction is writable.
func getTaskBucket(tx *boltdd.Tx, allocID, taskName string) (*boltdd.Bucket, error) {
	alloc, err := getAllocationBucket(tx, allocID)
	if err != nil {
		return nil, err
	}

	// Retrieve the specific task bucket
	w := tx.Writable()
	key := taskBucketName(taskName)
	task := alloc.Bucket(key)
	if task == nil {
		if !w {
			return nil, fmt.Errorf("Task bucket doesn't exist and transaction is not writable")
		}

		task, err = alloc.CreateBucket(key)
		if err != nil {
			return nil, err
		}
	}

	return task, nil
}

// PutDevicePluginState stores the device manager's plugin state or returns an
// error.
func (s *BoltStateDB) PutDevicePluginState(ps *dmstate.PluginState) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		// Retrieve the root device manager bucket
		devBkt, err := tx.CreateBucketIfNotExists(devManagerBucket)
		if err != nil {
			return err
		}

		return devBkt.Put(managerPluginStateKey, ps)
	})
}

// GetDevicePluginState stores the device manager's plugin state or returns an
// error.
func (s *BoltStateDB) GetDevicePluginState() (*dmstate.PluginState, error) {
	var ps *dmstate.PluginState

	err := s.db.View(func(tx *boltdd.Tx) error {
		devBkt := tx.Bucket(devManagerBucket)
		if devBkt == nil {
			// No state, return
			return nil
		}

		// Restore Plugin State if it exists
		ps = &dmstate.PluginState{}
		if err := devBkt.Get(managerPluginStateKey, ps); err != nil {
			if !boltdd.IsErrNotFound(err) {
				return fmt.Errorf("failed to read device manager plugin state: %v", err)
			}

			// Key not found, reset ps to nil
			ps = nil
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ps, nil
}

// PutDriverPluginState stores the driver manager's plugin state or returns an
// error.
func (s *BoltStateDB) PutDriverPluginState(ps *driverstate.PluginState) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		// Retrieve the root driver manager bucket
		driverBkt, err := tx.CreateBucketIfNotExists(driverManagerBucket)
		if err != nil {
			return err
		}

		return driverBkt.Put(managerPluginStateKey, ps)
	})
}

// GetDriverPluginState stores the driver manager's plugin state or returns an
// error.
func (s *BoltStateDB) GetDriverPluginState() (*driverstate.PluginState, error) {
	var ps *driverstate.PluginState

	err := s.db.View(func(tx *boltdd.Tx) error {
		driverBkt := tx.Bucket(driverManagerBucket)
		if driverBkt == nil {
			// No state, return
			return nil
		}

		// Restore Plugin State if it exists
		ps = &driverstate.PluginState{}
		if err := driverBkt.Get(managerPluginStateKey, ps); err != nil {
			if !boltdd.IsErrNotFound(err) {
				return fmt.Errorf("failed to read driver manager plugin state: %v", err)
			}

			// Key not found, reset ps to nil
			ps = nil
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ps, nil
}

// PutDynamicPluginRegistryState stores the dynamic plugin registry's
// state or returns an error.
func (s *BoltStateDB) PutDynamicPluginRegistryState(ps *dynamicplugins.RegistryState) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		// Retrieve the root dynamic plugin manager bucket
		dynamicBkt, err := tx.CreateBucketIfNotExists(dynamicPluginBucketName)
		if err != nil {
			return err
		}
		return dynamicBkt.Put(registryStateKey, ps)
	})
}

// GetDynamicPluginRegistryState stores the dynamic plugin registry's
// registry state or returns an error.
func (s *BoltStateDB) GetDynamicPluginRegistryState() (*dynamicplugins.RegistryState, error) {
	var ps *dynamicplugins.RegistryState

	err := s.db.View(func(tx *boltdd.Tx) error {
		dynamicBkt := tx.Bucket(dynamicPluginBucketName)
		if dynamicBkt == nil {
			// No state, return
			return nil
		}

		// Restore Plugin State if it exists
		ps = &dynamicplugins.RegistryState{}
		if err := dynamicBkt.Get(registryStateKey, ps); err != nil {
			if !boltdd.IsErrNotFound(err) {
				return fmt.Errorf("failed to read dynamic plugin registry state: %v", err)
			}

			// Key not found, reset ps to nil
			ps = nil
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ps, nil
}

func keyForCheck(allocID string, checkID structs.CheckID) []byte {
	return []byte(fmt.Sprintf("%s_%s", allocID, checkID))
}

// PutCheckResult puts qr into the state store.
func (s *BoltStateDB) PutCheckResult(allocID string, qr *structs.CheckQueryResult) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		bkt, err := tx.CreateBucketIfNotExists(checkResultsBucket)
		if err != nil {
			return err
		}
		key := keyForCheck(allocID, qr.ID)
		return bkt.Put(key, qr)
	})
}

// GetCheckResults gets the check results associated with allocID from the state store.
func (s *BoltStateDB) GetCheckResults() (checks.ClientResults, error) {
	m := make(checks.ClientResults)

	err := s.db.View(func(tx *boltdd.Tx) error {
		bkt := tx.Bucket(checkResultsBucket)
		if bkt == nil {
			return nil // nothing set yet
		}

		if err := boltdd.Iterate(bkt, nil, func(key []byte, qr structs.CheckQueryResult) {
			parts := bytes.SplitN(key, []byte("_"), 2)
			allocID, _ := parts[0], parts[1]
			m.Insert(string(allocID), &qr)
		}); err != nil {
			return err
		}
		return nil
	})
	return m, err
}

func (s *BoltStateDB) DeleteCheckResults(allocID string, checkIDs []structs.CheckID) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		bkt := tx.Bucket(checkResultsBucket)
		if bkt == nil {
			return nil // nothing set yet
		}

		for _, id := range checkIDs {
			key := keyForCheck(allocID, id)
			if err := bkt.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *BoltStateDB) PurgeCheckResults(allocID string) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		bkt := tx.Bucket(checkResultsBucket)
		if bkt == nil {
			return nil // nothing set yet
		}
		return bkt.DeletePrefix([]byte(allocID + "_"))
	})
}

// PutNodeMeta sets dynamic node metadata for merging with the copy from the
// Client's config.
//
// This overwrites existing dynamic node metadata entirely.
func (s *BoltStateDB) PutNodeMeta(meta map[string]*string) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		b, err := tx.CreateBucketIfNotExists(nodeMetaBucket)
		if err != nil {
			return err
		}

		return b.Put(nodeMetaKey, meta)
	})
}

// GetNodeMeta retrieves node metadata for merging with the copy from
// the Client's config.
func (s *BoltStateDB) GetNodeMeta() (m map[string]*string, err error) {
	err = s.db.View(func(tx *boltdd.Tx) error {
		b := tx.Bucket(nodeMetaBucket)
		if b == nil {
			return nil
		}

		m, err = getNodeMeta(b)
		return err
	})

	return m, err
}

func getNodeMeta(b *boltdd.Bucket) (map[string]*string, error) {
	m := make(map[string]*string)
	if err := b.Get(nodeMetaKey, m); err != nil {
		if !boltdd.IsErrNotFound(err) {
			return nil, err
		}
	}
	return m, nil
}

func (s *BoltStateDB) PutNodeRegistration(reg *cstructs.NodeRegistration) error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		b, err := tx.CreateBucketIfNotExists(nodeBucket)
		if err != nil {
			return err
		}

		return b.Put(nodeRegistrationKey, reg)
	})
}

func (s *BoltStateDB) GetNodeRegistration() (*cstructs.NodeRegistration, error) {
	var reg cstructs.NodeRegistration
	err := s.db.View(func(tx *boltdd.Tx) error {
		b := tx.Bucket(nodeBucket)
		if b == nil {
			return nil
		}
		return b.Get(nodeRegistrationKey, &reg)
	})

	if boltdd.IsErrNotFound(err) {
		return nil, nil
	}

	return &reg, err
}

// init initializes metadata entries in a newly created state database.
func (s *BoltStateDB) init() error {
	return s.db.Update(func(tx *boltdd.Tx) error {
		return addMeta(tx.BoltTx())
	})
}

// acknowledgedStateEntry wraps values in the acknowledged_state bucket, so we
// can expand it in the future if need be
type acknowledgedStateEntry struct {
	State *arstate.State
}

// updateWithOptions enables adjustments to db.Update operation, including Batch mode.
func (s *BoltStateDB) updateWithOptions(opts []WriteOption, updateFn func(tx *boltdd.Tx) error) error {
	writeOpts := mergeWriteOptions(opts)

	if writeOpts.BatchMode {
		// In Batch mode, BoltDB opportunistically combines multiple concurrent writes into one or
		// several transactions. See boltdb.Batch() documentation for details.
		return s.db.Batch(updateFn)
	} else {
		return s.db.Update(updateFn)
	}
}

// Upgrade bolt state db from 0.8 schema to 0.9 schema. Noop if already using
// 0.9 schema. Creates a backup before upgrading.
func (s *BoltStateDB) Upgrade() error {
	// Check to see if the underlying DB needs upgrading.
	upgrade09, upgrade13, err := NeedsUpgrade(s.db.BoltDB())
	if err != nil {
		return err
	}
	if !upgrade09 && !upgrade13 {
		// No upgrade needed!
		return nil
	}

	// Upgraded needed. Backup the boltdb first.
	backupFileName := filepath.Join(s.stateDir, "state.db.backup")
	if err := backupDB(s.db.BoltDB(), backupFileName); err != nil {
		return fmt.Errorf("error backing up state db: %v", err)
	}

	// Perform the upgrade
	if err := s.db.Update(func(tx *boltdd.Tx) error {

		if upgrade09 {
			if err := UpgradeAllocs(s.logger, tx); err != nil {
				return err
			}
		}
		if upgrade13 {
			if err := UpgradeDynamicPluginRegistry(s.logger, tx); err != nil {
				return err
			}
		}

		// Add standard metadata
		if err := addMeta(tx.BoltTx()); err != nil {
			return err
		}

		// Write the time the upgrade was done
		bkt, err := tx.CreateBucketIfNotExists(metaBucketName)
		if err != nil {
			return err
		}

		return bkt.Put(metaUpgradedKey, time.Now().Format(time.RFC3339))
	}); err != nil {
		return err
	}

	s.logger.Info("successfully upgraded state")
	return nil
}

// DB allows access to the underlying BoltDB for testing purposes.
func (s *BoltStateDB) DB() *boltdd.DB {
	return s.db
}
