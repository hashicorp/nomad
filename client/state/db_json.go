// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"sync"

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

	logger hclog.Logger

	file *os.File

	mu sync.RWMutex
}

type JsonDBMeta struct {
	Sha256sum []byte
}

func NewJsonDB(logger hclog.Logger, stateDir string) (StateDB, error) {
	// Open root
	root, err := os.OpenRoot(stateDir)
	if err != nil {
		return nil, fmt.Errorf("error opening state dir: %w", err)
	}

	// Open/create file
	fi, err := root.OpenFile("state.json", os.O_RDWR|os.O_CREATE, 0640)
	if err != nil {
		return nil, err
	}

	// Lock file
	if err := flock.FLock(fi); err != nil {
		return nil, err
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
	}
	db.logger = logger.Named(db.Name())

	// Decode client state
	dec := json.NewDecoder(fi)
	if err := dec.Decode(db); err != nil {
		if errors.Is(err, io.EOF) {
			// New file, nothing more to do
			return db, nil
		}

		// Unexpected error, funlock and return
		_ = flock.FUnlock(fi)
		_ = fi.Close()
		return nil, fmt.Errorf("unable to decode client state: %w", err)
	}

	// Record the end of the first object
	off := dec.InputOffset()

	// Decode checksum record
	meta := &JsonDBMeta{}
	if err := dec.Decode(meta); err != nil {
		return nil, fmt.Errorf("unable to decode client state metadata: %w", err)
	}
	if n := len(meta.Sha256sum); n != sha256.Size {
		if n == 0 {
			return nil, fmt.Errorf("no checksum for client state")
		} else {
			return nil, fmt.Errorf("client state checksum is the wrong size. expected: %d, found: %d", sha256.Size, n)
		}
	}
	hasher := sha256.New()

	// Rewind and hash first object
	if _, err := fi.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to rewind client state file: %s", err)
	}

	lr := &io.LimitedReader{
		R: fi,
		N: off,
	}
	if n, err := io.Copy(hasher, lr); err != nil {
		return nil, fmt.Errorf("error checksumming client state file: %w", err)
	} else if n != off {
		return nil, fmt.Errorf("unexpected amount of client state checksummed. expected: %d, found: %d", off, n)
	}

	// No need for constant time comparison as hash is only used to validate
	// file. Anyone able to udpate the file should also be able to update the
	// checksum.
	if !bytes.Equal(hasher.Sum(nil), meta.Sha256sum) {
		return nil, fmt.Errorf("client state file failed checksum. expected: %x, found: %x", meta.Sha256sum, hasher.Sum(nil))
	}

	db.file = fi

	return db, nil
}

func (db *JsonDB) save() error {
	// Encode
	buf, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return fmt.Errorf("error encoding client state as json: %w", err)
	}

	// Checksum
	sum := sha256.Sum256(buf)
	meta := &JsonDBMeta{
		Sha256sum: sum[:],
	}

	if _, err := db.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to rewind client state file: %w", err)
	}

	// Newline delimit records
	buf = append(buf, '\n')

	if _, err := db.file.Write(buf); err != nil {
		return fmt.Errorf("failed to write client state: %w", err)
	}
	if err := json.NewEncoder(db.file).Encode(meta); err != nil {
		return fmt.Errorf("failed to write client state metadata: %w", err)
	}

	if err := db.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync client state: %w", err)
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

	// Set everything to nil to blow up on further use
	db.Allocs = nil
	db.TaskState = nil
	db.LocalTaskState = nil

	return nil
}
