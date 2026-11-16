// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	arstate "github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	driverstate "github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/flock"
	"github.com/hashicorp/nomad/nomad/structs"
)

// JsonDB implements a StateDB that stores data in memory and should only be
// used for testing. All methods are safe for concurrent use.
type JsonDB struct {
	// alloc_id -> value
	Allocs map[string]*structs.Allocation

	// alloc_id -> value
	DeployState map[string]*structs.AllocDeploymentStatus

	// alloc_id -> value
	NetworkStatus map[string]*structs.AllocNetworkStatus

	// alloc_id -> value
	AckState map[string]*arstate.State

	// alloc_id -> value
	AllocVolStates map[string]*arstate.AllocVolumes

	// alloc_id -> task_name -> value
	LocalTaskState map[string]map[string]*state.LocalState
	TaskState      map[string]map[string]*structs.TaskState

	// alloc_id -> check_id -> result
	Checks checks.ClientResults

	// alloc_id -> []Identities
	Identities map[string][]*structs.SignedWorkloadIdentity

	// alloc_id -> []consulAclTokens
	ConsulACLTokens map[string][]*cstructs.ConsulACLToken

	// devicemanager -> plugin-state
	DevManagerPs *dmstate.PluginState

	// drivermanager -> plugin-state
	DriverManagerPs *driverstate.PluginState

	// dynamicmanager -> registry-state
	DynManagerPs *dynamicplugins.RegistryState

	// key -> value or nil
	NodeMeta map[string]*string

	NodeReg *cstructs.NodeRegistration

	DynHostVols map[string]*cstructs.HostVolumeState

	// ClientIdent is the persisted identity of the client.
	ClientIdent string

	lockFile *os.File
	root     *os.Root
	pid      int

	logger hclog.Logger

	mu sync.RWMutex
}

type JsonDBMeta struct {
	Sha256sum []byte
}

func NewJsonDB(logger hclog.Logger, stateDir string) (StateDB, error) {
	// Open root
	root, err := os.OpenRoot(stateDir)
	if err != nil {
		defer root.Close()
		return nil, fmt.Errorf("error opening state dir: %w", err)
	}

	// Open lock file
	lockFile, err := root.OpenFile("state.json.lock", os.O_RDWR|os.O_CREATE, 0640)
	if err != nil {
		defer root.Close()
		return nil, err
	}

	// Lock file
	if err := flock.FLock(lockFile); err != nil {
		defer root.Close()
		defer lockFile.Close()
		if errors.Is(err, flock.ErrLocked) {
			buf := make([]byte, 1000)
			n, ferr := lockFile.Read(buf)
			if ferr != nil {
				return nil, fmt.Errorf("client lock file %s locked. error reading: %w", lockFile.Name(), ferr)
			}
			return nil, fmt.Errorf("client lock file %s locked. contents: %q", lockFile.Name(), string(buf[0:n]))
		}
		return nil, err
	}

	// Write info for concurrent openers
	pid := os.Getpid()
	lockmsg := []byte(fmt.Sprintf("%d at %s", pid, time.Now()))
	if err := lockFile.Truncate(int64(len(lockmsg))); err != nil {
		defer root.Close()
		defer lockFile.Close()
		return nil, fmt.Errorf("error truncating client lock file: %w", err)
	}
	if _, err := lockFile.Seek(0, io.SeekStart); err != nil {
		defer root.Close()
		defer lockFile.Close()
		return nil, fmt.Errorf("error seeking in client lock file: %w", err)
	}
	if n, err := lockFile.Write(lockmsg); err != nil {
		defer root.Close()
		defer lockFile.Close()
		return nil, fmt.Errorf("error writing client lock file (%d bytes written): %w", n, err)
	}
	if err := lockFile.Sync(); err != nil {
		defer root.Close()
		defer lockFile.Close()
		return nil, fmt.Errorf("error syncing client lock file: %w", err)
	}

	// Initialize struct to empty values
	db := &JsonDB{
		Allocs:          make(map[string]*structs.Allocation),
		DeployState:     make(map[string]*structs.AllocDeploymentStatus),
		NetworkStatus:   make(map[string]*structs.AllocNetworkStatus),
		AckState:        make(map[string]*arstate.State),
		LocalTaskState:  make(map[string]map[string]*state.LocalState),
		TaskState:       make(map[string]map[string]*structs.TaskState),
		Checks:          make(checks.ClientResults),
		Identities:      make(map[string][]*structs.SignedWorkloadIdentity),
		ConsulACLTokens: make(map[string][]*cstructs.ConsulACLToken),
		DynHostVols:     make(map[string]*cstructs.HostVolumeState),
		lockFile:        lockFile,
		root:            root,
		pid:             pid,
	}
	db.logger = logger.Named(db.Name())

	if err := db.load(); err != nil {
		defer root.Close()
		return nil, err
	}

	return db, nil
}

func (db *JsonDB) load() error {
	stateFile, err := db.root.Open("state.json")
	if errors.Is(err, os.ErrNotExist) {
		// Nothing to load
		return nil
	}
	if err != nil {
		return fmt.Errorf("error loading client state: %w", err)
	}
	defer stateFile.Close()

	// Decode client state
	dec := json.NewDecoder(stateFile)
	if err := dec.Decode(db); err != nil {
		return fmt.Errorf("unable to decode client state: %w", err)
	}

	return nil
}

func (db *JsonDB) save() error {
	tmpfn := fmt.Sprintf("state.json.%d.%d", db.pid, time.Now().UnixMilli())
	stateFile, err := db.root.OpenFile(tmpfn, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("error opening client sate file for writing: %w", err)
	}

	enc := json.NewEncoder(stateFile)
	enc.SetIndent("", "  ")
	if err := enc.Encode(db); err != nil {
		_ = stateFile.Close()
		return fmt.Errorf("error writing client state to %q: %w", tmpfn, err)
	}

	if err := stateFile.Sync(); err != nil {
		return fmt.Errorf("error syncing client state file %q: %w", tmpfn, err)
	}

	if err := stateFile.Close(); err != nil {
		return fmt.Errorf("error closing client state file %q: %w", tmpfn, err)
	}

	if err := db.root.Rename(tmpfn, "state.json"); err != nil {
		return fmt.Errorf("error moving client state file: %w", err)
	}

	return nil
}

func (db *JsonDB) Name() string {
	return "jsondb"
}

func (db *JsonDB) Upgrade() error {
	return nil
}

func (db *JsonDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	allocs := make([]*structs.Allocation, 0, len(db.Allocs))
	for _, v := range db.Allocs {
		allocs = append(allocs, v)
	}

	return allocs, map[string]error{}, nil
}

func (db *JsonDB) PutAllocation(alloc *structs.Allocation, _ ...WriteOption) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.Allocs[alloc.ID] = alloc
	return db.save()
}

func (db *JsonDB) GetDeploymentStatus(allocID string) (*structs.AllocDeploymentStatus, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.DeployState[allocID], nil
}

func (db *JsonDB) PutDeploymentStatus(allocID string, ds *structs.AllocDeploymentStatus) error {
	db.mu.Lock()
	db.DeployState[allocID] = ds
	defer db.mu.Unlock()
	return db.save()
}

func (db *JsonDB) GetNetworkStatus(allocID string) (*structs.AllocNetworkStatus, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.NetworkStatus[allocID], nil
}

func (db *JsonDB) PutNetworkStatus(allocID string, ns *structs.AllocNetworkStatus, _ ...WriteOption) error {
	db.mu.Lock()
	db.NetworkStatus[allocID] = ns
	defer db.mu.Unlock()
	return db.save()
}

func (db *JsonDB) PutAcknowledgedState(allocID string, state *arstate.State, opts ...WriteOption) error {
	db.mu.Lock()
	db.AckState[allocID] = state
	defer db.mu.Unlock()
	return db.save()
}

func (db *JsonDB) GetAcknowledgedState(allocID string) (*arstate.State, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.AckState[allocID], nil
}

func (db *JsonDB) PutAllocVolumes(allocID string, state *arstate.AllocVolumes, opts ...WriteOption) error {
	db.mu.Lock()
	db.AllocVolStates[allocID] = state
	defer db.mu.Unlock()
	return db.save()
}

func (db *JsonDB) GetAllocVolumes(allocID string) (*arstate.AllocVolumes, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.AllocVolStates[allocID], nil
}

func (db *JsonDB) PutAllocIdentities(allocID string, identities []*structs.SignedWorkloadIdentity, _ ...WriteOption) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.Identities[allocID] = identities
	return db.save()
}

func (db *JsonDB) GetAllocIdentities(allocID string) ([]*structs.SignedWorkloadIdentity, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.Identities[allocID], nil
}

func (db *JsonDB) PutAllocConsulACLTokens(allocID string, tokens []*cstructs.ConsulACLToken, opts ...WriteOption) error {

	db.mu.Lock()
	defer db.mu.Unlock()
	db.ConsulACLTokens[allocID] = tokens
	return db.save()
}

func (db *JsonDB) GetAllocConsulACLTokens(allocID string) ([]*cstructs.ConsulACLToken, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.ConsulACLTokens[allocID], nil
}

func (db *JsonDB) GetTaskRunnerState(allocID string, taskName string) (*state.LocalState, *structs.TaskState, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var ls *state.LocalState
	var ts *structs.TaskState

	// Local Task State
	allocLocalTS := db.LocalTaskState[allocID]
	if len(allocLocalTS) != 0 {
		ls = allocLocalTS[taskName]
	}

	// Task State
	allocTS := db.TaskState[allocID]
	if len(allocTS) != 0 {
		ts = allocTS[taskName]
	}

	return ls, ts, nil
}

func (db *JsonDB) PutTaskRunnerLocalState(allocID string, taskName string, val *state.LocalState) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if alts, ok := db.LocalTaskState[allocID]; ok {
		alts[taskName] = val.Copy()
		return db.save()
	}

	db.LocalTaskState[allocID] = map[string]*state.LocalState{
		taskName: val.Copy(),
	}

	return db.save()
}

func (db *JsonDB) PutTaskState(allocID string, taskName string, state *structs.TaskState) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if ats, ok := db.TaskState[allocID]; ok {
		ats[taskName] = state.Copy()
		return db.save()
	}

	db.TaskState[allocID] = map[string]*structs.TaskState{
		taskName: state.Copy(),
	}

	return db.save()
}

func (db *JsonDB) DeleteTaskBucket(allocID, taskName string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if ats, ok := db.TaskState[allocID]; ok {
		delete(ats, taskName)
	}

	if alts, ok := db.LocalTaskState[allocID]; ok {
		delete(alts, taskName)
	}

	return db.save()
}

func (db *JsonDB) DeleteAllocationBucket(allocID string, _ ...WriteOption) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	delete(db.Allocs, allocID)
	delete(db.TaskState, allocID)
	delete(db.LocalTaskState, allocID)
	delete(db.Identities, allocID)

	return db.save()
}

func (db *JsonDB) PutDevicePluginState(ps *dmstate.PluginState) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.DevManagerPs = ps
	return db.save()
}

// GetDevicePluginState stores the device manager's plugin state or returns an
// error.
func (db *JsonDB) GetDevicePluginState() (*dmstate.PluginState, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.DevManagerPs, nil
}

func (db *JsonDB) GetDriverPluginState() (*driverstate.PluginState, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.DriverManagerPs, nil
}

func (db *JsonDB) PutDriverPluginState(ps *driverstate.PluginState) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.DriverManagerPs = ps
	return db.save()
}

func (db *JsonDB) GetDynamicPluginRegistryState() (*dynamicplugins.RegistryState, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.DynManagerPs, nil
}

func (db *JsonDB) PutDynamicPluginRegistryState(ps *dynamicplugins.RegistryState) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.DynManagerPs = ps
	return db.save()
}

func (db *JsonDB) PutCheckResult(allocID string, qr *structs.CheckQueryResult) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.Checks[allocID]; !exists {
		db.Checks[allocID] = make(checks.AllocationResults)
	}

	db.Checks[allocID][qr.ID] = qr
	return db.save()
}

func (db *JsonDB) GetCheckResults() (checks.ClientResults, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return maps.Clone(db.Checks), nil
}

func (db *JsonDB) DeleteCheckResults(allocID string, checkIDs []structs.CheckID) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	for _, id := range checkIDs {
		delete(db.Checks[allocID], id)
	}
	return db.save()
}

func (db *JsonDB) PurgeCheckResults(allocID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.Checks, allocID)
	return db.save()
}

func (db *JsonDB) PutNodeMeta(nm map[string]*string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.NodeMeta = nm
	return db.save()
}

func (db *JsonDB) GetNodeMeta() (map[string]*string, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.NodeMeta, nil
}

func (db *JsonDB) PutNodeRegistration(reg *cstructs.NodeRegistration) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.NodeReg = reg
	return db.save()
}

func (db *JsonDB) GetNodeRegistration() (*cstructs.NodeRegistration, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.NodeReg, nil
}

func (db *JsonDB) PutDynamicHostVolume(vol *cstructs.HostVolumeState) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.DynHostVols[vol.ID] = vol
	return db.save()
}
func (db *JsonDB) GetDynamicHostVolumes() ([]*cstructs.HostVolumeState, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	var vols []*cstructs.HostVolumeState
	for _, vol := range db.DynHostVols {
		vols = append(vols, vol)
	}
	return vols, nil
}
func (db *JsonDB) DeleteDynamicHostVolume(s string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.DynHostVols, s)
	return db.save()
}

func (db *JsonDB) PutNodeIdentity(identity string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.ClientIdent = identity
	return db.save()
}

func (db *JsonDB) GetNodeIdentity() (string, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.ClientIdent, nil
}

func (db *JsonDB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if err := db.save(); err != nil {
		return fmt.Errorf("error saving before close: %w", err)
	}

	_ = flock.FUnlock(db.lockFile)
	_ = db.lockFile.Close()
	_ = db.root.Close()

	// Set everything to nil to blow up on further use
	db.Allocs = nil
	db.TaskState = nil
	db.LocalTaskState = nil

	return nil
}
