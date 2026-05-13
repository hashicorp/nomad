// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	hclog "github.com/hashicorp/go-hclog"
	arstate "github.com/hashicorp/nomad/client/allocrunner/state"
	trstate "github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	driverstate "github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"

	_ "modernc.org/sqlite"
)

// sqliteSchemaVersion is the current version of the SQLite state schema. Bump
// this when making incompatible schema changes and add a migration in Upgrade.
const sqliteSchemaVersion = 1

// Keys used in alloc_state for per-allocation data.
const (
	sqlKeyAlloc                = "alloc"
	sqlKeyDeployStatus         = "deploy_status"
	sqlKeyNetworkStatus        = "network_status"
	sqlKeyAcknowledgedState    = "acknowledged_state"
	sqlKeyAllocVolumes         = "alloc_volumes"
	sqlKeyAllocIdentities      = "alloc_identities"
	sqlKeyAllocConsulACLTokens = "alloc_consul_acl_tokens"
)

// Keys used in task_state for per-task data.
const (
	sqlKeyTaskLocalState = "local_state"
	sqlKeyTaskState      = "task_state"
)

// Keys used in kv_state for singleton / global state.
const (
	sqlKeyDevicePluginState          = "device_plugin_state"
	sqlKeyDriverPluginState          = "driver_plugin_state"
	sqlKeyDynamicPluginRegistryState = "dynamic_plugin_registry_state"
	sqlKeyNodeMeta                   = "node_meta"
	sqlKeyNodeRegistration           = "node_registration"
	sqlKeyNodeIdentity               = "node_identity"
)

// createSchemaSQL is the DDL executed (idempotently) on every open to ensure
// the schema is in place. All CREATE TABLE statements use IF NOT EXISTS, so
// calling this on an already-initialised database is a no-op.
const createSchemaSQL = `
PRAGMA journal_mode = WAL;
PRAGMA synchronous  = NORMAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;

-- Schema version tracking.
CREATE TABLE IF NOT EXISTS schema_meta (
    key   TEXT PRIMARY KEY NOT NULL,
    value TEXT NOT NULL
);

-- Per-allocation key-value blobs (alloc data, deploy/network status, etc.).
CREATE TABLE IF NOT EXISTS alloc_state (
    alloc_id TEXT NOT NULL,
    key      TEXT NOT NULL,
    value    BLOB NOT NULL,
    PRIMARY KEY (alloc_id, key)
);

-- Per-task key-value blobs (local state, task state).
CREATE TABLE IF NOT EXISTS task_state (
    alloc_id  TEXT NOT NULL,
    task_name TEXT NOT NULL,
    key       TEXT NOT NULL,
    value     BLOB NOT NULL,
    PRIMARY KEY (alloc_id, task_name, key)
);

-- Check query results, indexed for fast alloc-level purge.
CREATE TABLE IF NOT EXISTS check_results (
    alloc_id TEXT NOT NULL,
    check_id TEXT NOT NULL,
    value    BLOB NOT NULL,
    PRIMARY KEY (alloc_id, check_id)
);

-- Dynamic host volume state, each row is one volume.
CREATE TABLE IF NOT EXISTS host_volumes (
    vol_id TEXT PRIMARY KEY NOT NULL,
    value  BLOB NOT NULL
);

-- Singleton / global key-value blobs (managers, node meta, etc.).
CREATE TABLE IF NOT EXISTS kv_state (
    key   TEXT PRIMARY KEY NOT NULL,
    value BLOB NOT NULL
);

INSERT OR IGNORE INTO schema_meta (key, value)
    VALUES ('schema_version', '1');
`

// SQLiteStateDB persists and restores Nomad client state in a SQLite database.
// All public methods are safe for concurrent access; internally a single
// connection is used (SetMaxOpenConns(1)) so SQLite's serialised write mode
// applies and "database is locked" errors are avoided.
type SQLiteStateDB struct {
	stateDir string
	db       *sql.DB
	logger   hclog.Logger
}

var _ StateDB = (*SQLiteStateDB)(nil)

// NewSQLiteStateDB creates or opens a SQLite-backed state database rooted at
// stateDir. The file is named "state.db" inside that directory.
func NewSQLiteStateDB(logger hclog.Logger, stateDir string) (StateDB, error) {
	fn := filepath.Join(stateDir, "state.db")

	// Check whether the file exists before opening so we can log first-run.
	fi, err := os.Stat(fn)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to stat state database: %w", err)
	}
	firstRun := fi == nil

	db, err := sql.Open("sqlite", fn)
	if err != nil {
		return nil, fmt.Errorf("failed to open state database: %w", err)
	}

	// Use a single connection pool entry. SQLite supports only one writer at
	// a time; serialising through one connection avoids "database is locked"
	// errors without sacrificing meaningful concurrency for a local agent.
	db.SetMaxOpenConns(1)

	sdb := &SQLiteStateDB{
		stateDir: stateDir,
		db:       db,
		logger:   logger.Named("sqlite_state_db"),
	}

	if err := sdb.createSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialise state database schema: %w", err)
	}

	if firstRun {
		sdb.logger.Info("created new SQLite state database", "path", fn)
	} else {
		sdb.logger.Info("opened existing SQLite state database", "path", fn)
	}

	return sdb, nil
}

// createSchema applies the DDL. Because every CREATE TABLE uses IF NOT EXISTS,
// this is idempotent and safe to call on an already-populated database.
func (s *SQLiteStateDB) createSchema() error {
	_, err := s.db.Exec(createSchemaSQL)
	if err != nil {
		return fmt.Errorf("schema DDL failed: %w", err)
	}
	return nil
}

// Name implements StateDB.
func (s *SQLiteStateDB) Name() string { return "sqlite" }

// Upgrade implements StateDB. For a freshly-created SQLite database the schema
// is already current. Future schema versions should add migration logic here.
func (s *SQLiteStateDB) Upgrade() error {
	return s.createSchema()
}

// Close implements StateDB.
func (s *SQLiteStateDB) Close() error { return s.db.Close() }

// ---------------------------------------------------------------------------
// Serialisation helpers
// ---------------------------------------------------------------------------

// sqlMarshal encodes v to JSON for storage. Using encoding/json keeps the
// implementation dependency-free; all Nomad state types are JSON-safe.
func sqlMarshal(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %w", err)
	}
	return b, nil
}

// sqlUnmarshal decodes JSON data into v.
func sqlUnmarshal(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unmarshal failed: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Low-level helpers for the three main table patterns
// ---------------------------------------------------------------------------

// putKV upserts a singleton value into kv_state.
func (s *SQLiteStateDB) putKV(key string, val any) error {
	data, err := sqlMarshal(val)
	if err != nil {
		return fmt.Errorf("serialize kv key=%q: %w", key, err)
	}
	_, err = s.db.Exec(
		`INSERT INTO kv_state (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, data,
	)
	return err
}

// getKV retrieves a singleton value from kv_state.
// Returns (false, nil) when the key does not exist.
func (s *SQLiteStateDB) getKV(key string, v any) (bool, error) {
	var data []byte
	err := s.db.QueryRow(`SELECT value FROM kv_state WHERE key = ?`, key).Scan(&data)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, sqlUnmarshal(data, v)
}

// putAllocState upserts a keyed blob for a specific allocation.
func (s *SQLiteStateDB) putAllocState(allocID, key string, val any) error {
	data, err := sqlMarshal(val)
	if err != nil {
		return fmt.Errorf("serialize alloc state alloc=%q key=%q: %w", allocID, key, err)
	}
	_, err = s.db.Exec(
		`INSERT INTO alloc_state (alloc_id, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(alloc_id, key) DO UPDATE SET value = excluded.value`,
		allocID, key, data,
	)
	return err
}

// getAllocState retrieves a keyed blob for a specific allocation.
// Returns (false, nil) when the row does not exist.
func (s *SQLiteStateDB) getAllocState(allocID, key string, v any) (bool, error) {
	var data []byte
	err := s.db.QueryRow(
		`SELECT value FROM alloc_state WHERE alloc_id = ? AND key = ?`,
		allocID, key,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, sqlUnmarshal(data, v)
}

// putTaskState upserts a keyed blob for a specific task.
func (s *SQLiteStateDB) putTaskStateRow(allocID, taskName, key string, val any) error {
	data, err := sqlMarshal(val)
	if err != nil {
		return fmt.Errorf("serialize task state alloc=%q task=%q key=%q: %w", allocID, taskName, key, err)
	}
	_, err = s.db.Exec(
		`INSERT INTO task_state (alloc_id, task_name, key, value) VALUES (?, ?, ?, ?)
		 ON CONFLICT(alloc_id, task_name, key) DO UPDATE SET value = excluded.value`,
		allocID, taskName, key, data,
	)
	return err
}

// getTaskStateRow retrieves a keyed blob for a specific task.
// Returns (false, nil) when the row does not exist.
func (s *SQLiteStateDB) getTaskStateRow(allocID, taskName, key string, v any) (bool, error) {
	var data []byte
	err := s.db.QueryRow(
		`SELECT value FROM task_state WHERE alloc_id = ? AND task_name = ? AND key = ?`,
		allocID, taskName, key,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, sqlUnmarshal(data, v)
}

// ---------------------------------------------------------------------------
// Allocation methods
// ---------------------------------------------------------------------------

// GetAllAllocations implements StateDB.
func (s *SQLiteStateDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	rows, err := s.db.Query(
		`SELECT alloc_id, value FROM alloc_state WHERE key = ?`, sqlKeyAlloc,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("query allocations: %w", err)
	}
	defer rows.Close()

	allocs := make([]*structs.Allocation, 0)
	errs := make(map[string]error)

	for rows.Next() {
		var allocID string
		var data []byte
		if err := rows.Scan(&allocID, &data); err != nil {
			return nil, nil, fmt.Errorf("scan allocation row: %w", err)
		}
		var alloc structs.Allocation
		if err := sqlUnmarshal(data, &alloc); err != nil {
			errs[allocID] = fmt.Errorf("failed to decode alloc: %w", err)
			continue
		}
		alloc.Canonicalize()
		if alloc.Job != nil {
			alloc.Job.Canonicalize()
		}
		allocs = append(allocs, &alloc)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate allocations: %w", err)
	}

	return allocs, errs, nil
}

// PutAllocation implements StateDB.
func (s *SQLiteStateDB) PutAllocation(alloc *structs.Allocation, opts ...WriteOption) error {
	return s.putAllocState(alloc.ID, sqlKeyAlloc, alloc)
}

// GetDeploymentStatus implements StateDB.
func (s *SQLiteStateDB) GetDeploymentStatus(allocID string) (*structs.AllocDeploymentStatus, error) {
	var ds structs.AllocDeploymentStatus
	found, err := s.getAllocState(allocID, sqlKeyDeployStatus, &ds)
	if !found || err != nil {
		return nil, err
	}
	return &ds, nil
}

// PutDeploymentStatus implements StateDB.
func (s *SQLiteStateDB) PutDeploymentStatus(allocID string, ds *structs.AllocDeploymentStatus) error {
	return s.putAllocState(allocID, sqlKeyDeployStatus, ds)
}

// GetNetworkStatus implements StateDB.
func (s *SQLiteStateDB) GetNetworkStatus(allocID string) (*structs.AllocNetworkStatus, error) {
	var ns structs.AllocNetworkStatus
	found, err := s.getAllocState(allocID, sqlKeyNetworkStatus, &ns)
	if !found || err != nil {
		return nil, err
	}
	return &ns, nil
}

// PutNetworkStatus implements StateDB.
func (s *SQLiteStateDB) PutNetworkStatus(allocID string, ns *structs.AllocNetworkStatus, opts ...WriteOption) error {
	return s.putAllocState(allocID, sqlKeyNetworkStatus, ns)
}

// GetAcknowledgedState implements StateDB.
func (s *SQLiteStateDB) GetAcknowledgedState(allocID string) (*arstate.State, error) {
	var st arstate.State
	found, err := s.getAllocState(allocID, sqlKeyAcknowledgedState, &st)
	if !found || err != nil {
		return nil, err
	}
	return &st, nil
}

// PutAcknowledgedState implements StateDB.
func (s *SQLiteStateDB) PutAcknowledgedState(allocID string, state *arstate.State, opts ...WriteOption) error {
	return s.putAllocState(allocID, sqlKeyAcknowledgedState, state)
}

// GetAllocVolumes implements StateDB.
func (s *SQLiteStateDB) GetAllocVolumes(allocID string) (*arstate.AllocVolumes, error) {
	var vols arstate.AllocVolumes
	found, err := s.getAllocState(allocID, sqlKeyAllocVolumes, &vols)
	if !found || err != nil {
		return nil, err
	}
	return &vols, nil
}

// PutAllocVolumes implements StateDB.
func (s *SQLiteStateDB) PutAllocVolumes(allocID string, state *arstate.AllocVolumes, opts ...WriteOption) error {
	return s.putAllocState(allocID, sqlKeyAllocVolumes, state)
}

// GetAllocIdentities implements StateDB.
func (s *SQLiteStateDB) GetAllocIdentities(allocID string) ([]*structs.SignedWorkloadIdentity, error) {
	var ids []*structs.SignedWorkloadIdentity
	found, err := s.getAllocState(allocID, sqlKeyAllocIdentities, &ids)
	if !found || err != nil {
		return nil, err
	}
	return ids, nil
}

// PutAllocIdentities implements StateDB.
func (s *SQLiteStateDB) PutAllocIdentities(allocID string, identities []*structs.SignedWorkloadIdentity, opts ...WriteOption) error {
	return s.putAllocState(allocID, sqlKeyAllocIdentities, identities)
}

// GetAllocConsulACLTokens implements StateDB.
func (s *SQLiteStateDB) GetAllocConsulACLTokens(allocID string) ([]*cstructs.ConsulACLToken, error) {
	var tokens []*cstructs.ConsulACLToken
	found, err := s.getAllocState(allocID, sqlKeyAllocConsulACLTokens, &tokens)
	if !found || err != nil {
		return nil, err
	}
	return tokens, nil
}

// PutAllocConsulACLTokens implements StateDB.
func (s *SQLiteStateDB) PutAllocConsulACLTokens(allocID string, tokens []*cstructs.ConsulACLToken, opts ...WriteOption) error {
	return s.putAllocState(allocID, sqlKeyAllocConsulACLTokens, tokens)
}

// DeleteAllocationBucket implements StateDB. It removes all rows associated
// with the given allocation in a single transaction.
func (s *SQLiteStateDB) DeleteAllocationBucket(allocID string, opts ...WriteOption) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for _, stmt := range []string{
		`DELETE FROM alloc_state   WHERE alloc_id = ?`,
		`DELETE FROM task_state    WHERE alloc_id = ?`,
		`DELETE FROM check_results WHERE alloc_id = ?`,
	} {
		if _, err := tx.Exec(stmt, allocID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------
// Task methods
// ---------------------------------------------------------------------------

// GetTaskRunnerState implements StateDB.
func (s *SQLiteStateDB) GetTaskRunnerState(allocID, taskName string) (*trstate.LocalState, *structs.TaskState, error) {
	var ls *trstate.LocalState
	var ts *structs.TaskState

	var localState trstate.LocalState
	found, err := s.getTaskStateRow(allocID, taskName, sqlKeyTaskLocalState, &localState)
	if err != nil {
		return nil, nil, fmt.Errorf("read local task runner state: %w", err)
	}
	if found {
		ls = &localState
	}

	var taskState structs.TaskState
	found, err = s.getTaskStateRow(allocID, taskName, sqlKeyTaskState, &taskState)
	if err != nil {
		return nil, nil, fmt.Errorf("read task state: %w", err)
	}
	if found {
		ts = &taskState
	}

	return ls, ts, nil
}

// PutTaskRunnerLocalState implements StateDB.
func (s *SQLiteStateDB) PutTaskRunnerLocalState(allocID, taskName string, val *trstate.LocalState) error {
	return s.putTaskStateRow(allocID, taskName, sqlKeyTaskLocalState, val)
}

// PutTaskState implements StateDB.
func (s *SQLiteStateDB) PutTaskState(allocID, taskName string, state *structs.TaskState) error {
	return s.putTaskStateRow(allocID, taskName, sqlKeyTaskState, state)
}

// DeleteTaskBucket implements StateDB.
func (s *SQLiteStateDB) DeleteTaskBucket(allocID, taskName string) error {
	_, err := s.db.Exec(
		`DELETE FROM task_state WHERE alloc_id = ? AND task_name = ?`,
		allocID, taskName,
	)
	return err
}

// ---------------------------------------------------------------------------
// Plugin-manager state methods
// ---------------------------------------------------------------------------

// GetDevicePluginState implements StateDB.
func (s *SQLiteStateDB) GetDevicePluginState() (*dmstate.PluginState, error) {
	var ps dmstate.PluginState
	found, err := s.getKV(sqlKeyDevicePluginState, &ps)
	if !found || err != nil {
		return nil, err
	}
	return &ps, nil
}

// PutDevicePluginState implements StateDB.
func (s *SQLiteStateDB) PutDevicePluginState(state *dmstate.PluginState) error {
	return s.putKV(sqlKeyDevicePluginState, state)
}

// GetDriverPluginState implements StateDB.
func (s *SQLiteStateDB) GetDriverPluginState() (*driverstate.PluginState, error) {
	var ps driverstate.PluginState
	found, err := s.getKV(sqlKeyDriverPluginState, &ps)
	if !found || err != nil {
		return nil, err
	}
	return &ps, nil
}

// PutDriverPluginState implements StateDB.
func (s *SQLiteStateDB) PutDriverPluginState(state *driverstate.PluginState) error {
	return s.putKV(sqlKeyDriverPluginState, state)
}

// GetDynamicPluginRegistryState implements StateDB.
//
// Note: dynamicplugins.RegistryState.Plugins contains container/list.List
// values, which have no exported fields. JSON (and msgpack) will serialise
// them as empty objects. On restoration the lists are empty; running plugin
// tasks re-register themselves through RegisterPlugin, so this is acceptable.
func (s *SQLiteStateDB) GetDynamicPluginRegistryState() (*dynamicplugins.RegistryState, error) {
	var rs dynamicplugins.RegistryState
	found, err := s.getKV(sqlKeyDynamicPluginRegistryState, &rs)
	if !found || err != nil {
		return nil, err
	}
	return &rs, nil
}

// PutDynamicPluginRegistryState implements StateDB.
func (s *SQLiteStateDB) PutDynamicPluginRegistryState(state *dynamicplugins.RegistryState) error {
	return s.putKV(sqlKeyDynamicPluginRegistryState, state)
}

// ---------------------------------------------------------------------------
// Check-result methods
// ---------------------------------------------------------------------------

// PutCheckResult implements StateDB.
func (s *SQLiteStateDB) PutCheckResult(allocID string, qr *structs.CheckQueryResult) error {
	data, err := sqlMarshal(qr)
	if err != nil {
		return fmt.Errorf("serialize check result: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO check_results (alloc_id, check_id, value) VALUES (?, ?, ?)
		 ON CONFLICT(alloc_id, check_id) DO UPDATE SET value = excluded.value`,
		allocID, string(qr.ID), data,
	)
	return err
}

// GetCheckResults implements StateDB.
func (s *SQLiteStateDB) GetCheckResults() (checks.ClientResults, error) {
	rows, err := s.db.Query(`SELECT alloc_id, value FROM check_results`)
	if err != nil {
		return nil, fmt.Errorf("query check results: %w", err)
	}
	defer rows.Close()

	m := make(checks.ClientResults)
	for rows.Next() {
		var allocID string
		var data []byte
		if err := rows.Scan(&allocID, &data); err != nil {
			return nil, fmt.Errorf("scan check result row: %w", err)
		}
		var qr structs.CheckQueryResult
		if err := sqlUnmarshal(data, &qr); err != nil {
			return nil, fmt.Errorf("decode check result: %w", err)
		}
		m.Insert(allocID, &qr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate check results: %w", err)
	}
	return m, nil
}

// DeleteCheckResults implements StateDB.
func (s *SQLiteStateDB) DeleteCheckResults(allocID string, checkIDs []structs.CheckID) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for _, id := range checkIDs {
		if _, err := tx.Exec(
			`DELETE FROM check_results WHERE alloc_id = ? AND check_id = ?`,
			allocID, string(id),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PurgeCheckResults implements StateDB.
func (s *SQLiteStateDB) PurgeCheckResults(allocID string) error {
	_, err := s.db.Exec(`DELETE FROM check_results WHERE alloc_id = ?`, allocID)
	return err
}

// ---------------------------------------------------------------------------
// Node-level state methods
// ---------------------------------------------------------------------------

// GetNodeMeta implements StateDB.
func (s *SQLiteStateDB) GetNodeMeta() (map[string]*string, error) {
	m := make(map[string]*string)
	found, err := s.getKV(sqlKeyNodeMeta, &m)
	if !found || err != nil {
		return nil, err
	}
	return m, nil
}

// PutNodeMeta implements StateDB.
func (s *SQLiteStateDB) PutNodeMeta(meta map[string]*string) error {
	return s.putKV(sqlKeyNodeMeta, meta)
}

// GetNodeRegistration implements StateDB.
func (s *SQLiteStateDB) GetNodeRegistration() (*cstructs.NodeRegistration, error) {
	var reg cstructs.NodeRegistration
	found, err := s.getKV(sqlKeyNodeRegistration, &reg)
	if !found || err != nil {
		return nil, err
	}
	return &reg, nil
}

// PutNodeRegistration implements StateDB.
func (s *SQLiteStateDB) PutNodeRegistration(reg *cstructs.NodeRegistration) error {
	return s.putKV(sqlKeyNodeRegistration, reg)
}

// GetNodeIdentity implements StateDB.
func (s *SQLiteStateDB) GetNodeIdentity() (string, error) {
	var identity string
	found, err := s.getKV(sqlKeyNodeIdentity, &identity)
	if !found || err != nil {
		return "", err
	}
	return identity, nil
}

// PutNodeIdentity implements StateDB.
func (s *SQLiteStateDB) PutNodeIdentity(identity string) error {
	return s.putKV(sqlKeyNodeIdentity, identity)
}

// ---------------------------------------------------------------------------
// Dynamic host volume methods
// ---------------------------------------------------------------------------

// PutDynamicHostVolume implements StateDB.
func (s *SQLiteStateDB) PutDynamicHostVolume(vol *cstructs.HostVolumeState) error {
	data, err := sqlMarshal(vol)
	if err != nil {
		return fmt.Errorf("serialize host volume: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO host_volumes (vol_id, value) VALUES (?, ?)
		 ON CONFLICT(vol_id) DO UPDATE SET value = excluded.value`,
		vol.ID, data,
	)
	return err
}

// GetDynamicHostVolumes implements StateDB.
func (s *SQLiteStateDB) GetDynamicHostVolumes() ([]*cstructs.HostVolumeState, error) {
	rows, err := s.db.Query(`SELECT value FROM host_volumes`)
	if err != nil {
		return nil, fmt.Errorf("query host volumes: %w", err)
	}
	defer rows.Close()

	var vols []*cstructs.HostVolumeState
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan host volume row: %w", err)
		}
		var vol cstructs.HostVolumeState
		if err := sqlUnmarshal(data, &vol); err != nil {
			return nil, fmt.Errorf("decode host volume: %w", err)
		}
		vols = append(vols, &vol)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate host volumes: %w", err)
	}
	return vols, nil
}

// DeleteDynamicHostVolume implements StateDB.
func (s *SQLiteStateDB) DeleteDynamicHostVolume(id string) error {
	_, err := s.db.Exec(`DELETE FROM host_volumes WHERE vol_id = ?`, id)
	return err
}
