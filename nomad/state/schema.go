// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state/indexer"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	tableIndex = "index"

	TableNamespaces           = "namespaces"
	TableNodePools            = "node_pools"
	TableServiceRegistrations = "service_registrations"
	TableVariables            = "variables"
	TableVariablesQuotas      = "variables_quota"
	TableRootKeys             = "root_keys"
	TableACLRoles             = "acl_roles"
	TableACLAuthMethods       = "acl_auth_methods"
	TableACLBindingRules      = "acl_binding_rules"
	TableAllocs               = "allocs"
	TableJobSubmission        = "job_submission"
)

const (
	indexID            = "id"
	indexJob           = "job"
	indexNodeID        = "node_id"
	indexAllocID       = "alloc_id"
	indexServiceName   = "service_name"
	indexExpiresGlobal = "expires-global"
	indexExpiresLocal  = "expires-local"
	indexKeyID         = "key_id"
	indexPath          = "path"
	indexName          = "name"
	indexSigningKey    = "signing_key"
	indexAuthMethod    = "auth_method"
)

var (
	schemaFactories SchemaFactories
	factoriesLock   sync.Mutex
)

// SchemaFactory is the factory method for returning a TableSchema
type SchemaFactory func() *memdb.TableSchema
type SchemaFactories []SchemaFactory

// RegisterSchemaFactories is used to register a table schema.
func RegisterSchemaFactories(factories ...SchemaFactory) {
	factoriesLock.Lock()
	defer factoriesLock.Unlock()
	schemaFactories = append(schemaFactories, factories...)
}

func GetFactories() SchemaFactories {
	return schemaFactories
}

func init() {
	// Register all schemas
	RegisterSchemaFactories([]SchemaFactory{
		indexTableSchema,
		nodeTableSchema,
		nodePoolTableSchema,
		jobTableSchema,
		jobSummarySchema,
		jobVersionSchema,
		jobSubmissionSchema,
		deploymentSchema,
		periodicLaunchTableSchema,
		evalTableSchema,
		allocTableSchema,
		vaultAccessorTableSchema,
		siTokenAccessorTableSchema,
		aclPolicyTableSchema,
		aclTokenTableSchema,
		oneTimeTokenTableSchema,
		autopilotConfigTableSchema,
		schedulerConfigTableSchema,
		clusterMetaTableSchema,
		csiVolumeTableSchema,
		csiPluginTableSchema,
		scalingPolicyTableSchema,
		scalingEventTableSchema,
		namespaceTableSchema,
		serviceRegistrationsTableSchema,
		variablesTableSchema,
		variablesQuotasTableSchema,
		wrappedRootKeySchema,
		aclRolesTableSchema,
		aclAuthMethodsTableSchema,
		bindingRulesTableSchema,
	}...)
}

// stateStoreSchema is used to return the schema for the state store
func stateStoreSchema() *memdb.DBSchema {
	// Create the root DB schema
	db := &memdb.DBSchema{
		Tables: make(map[string]*memdb.TableSchema),
	}

	// Add each of the tables
	for _, schemaFn := range GetFactories() {
		schema := schemaFn()
		if _, ok := db.Tables[schema.Name]; ok {
			panic(fmt.Sprintf("duplicate table name: %s", schema.Name))
		}
		db.Tables[schema.Name] = schema
	}
	return db
}

// indexTableSchema is used for tracking the most recent index used for each table.
func indexTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "index",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Key",
					Lowercase: true,
				},
			},
		},
	}
}

// nodeTableSchema returns the MemDB schema for the nodes table.
// This table is used to store all the client nodes that are registered.
func nodeTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "nodes",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is used for node management
			// and simple direct lookup. ID is required to be
			// unique.
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			"secret_id": {
				Name:         "secret_id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "SecretID",
				},
			},
			"node_pool": {
				Name:         "node_pool",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "NodePool",
				},
			},
		},
	}
}

// nodePoolTableSchema returns the MemDB schema for the node pools table.
// This table is used to store all the node pools registered in the cluster.
func nodePoolTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableNodePools,
		Indexes: map[string]*memdb.IndexSchema{
			// Name is the primary index used for lookup and is required to be
			// unique.
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Name",
				},
			},
		},
	}
}

// jobTableSchema returns the MemDB schema for the jobs table.
// This table is used to store all the jobs that have been submitted.
func jobTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "jobs",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is used for job management
			// and simple direct lookup. ID is required to be
			// unique within a namespace.
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// Use a compound index so the tuple of (Namespace, ID) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
				},
			},
			"type": {
				Name:         "type",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Type",
					Lowercase: false,
				},
			},
			"gc": {
				Name:         "gc",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.ConditionalIndex{
					Conditional: jobIsGCable,
				},
			},
			"periodic": {
				Name:         "periodic",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.ConditionalIndex{
					Conditional: jobIsPeriodic,
				},
			},
			"pool": {
				Name:         "pool",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "NodePool",
				},
			},
			// ModifyIndex allows sorting by last-changed
			"modify_index": {
				Name:         "modify_index",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UintFieldIndex{
					Field: "ModifyIndex",
				},
			},
		},
	}
}

// jobSummarySchema returns the memdb schema for the job summary table
func jobSummarySchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "job_summary",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// Use a compound index so the tuple of (Namespace, JobID) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "JobID",
						},
					},
				},
			},
		},
	}
}

// jobVersionSchema returns the memdb schema for the job version table which
// keeps a historical view of job versions.
func jobVersionSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "job_version",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// Use a compound index so the tuple of (Namespace, ID, Version) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field:     "ID",
							Lowercase: true,
						},

						&memdb.UintFieldIndex{
							Field: "Version",
						},
					},
				},
			},
		},
	}
}

// jobSubmissionSchema returns the memdb table schema of job submissions
// which contain the original source material of each job, per version.
func jobSubmissionSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableJobSubmission,
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				// index by (Namespace, JobID, Version)
				// note: uniqueness applies only at the moment of insertion,
				// if anything modifies one of these fields (as the stored
				// struct is a pointer, there is no consistency)
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field:     "JobID",
							Lowercase: true,
						},

						&memdb.UintFieldIndex{
							Field: "Version",
						},
					},
				},
			},
		},
	}
}

// jobIsGCable satisfies the ConditionalIndexFunc interface and creates an index
// on whether a job is eligible for garbage collection.
func jobIsGCable(obj interface{}) (bool, error) {
	j, ok := obj.(*structs.Job)
	if !ok {
		return false, fmt.Errorf("Unexpected type: %v", obj)
	}

	// job versions that are tagged should be kept
	if j.VersionTag != nil {
		return false, nil
	}

	// If the job is periodic or parameterized it is only garbage collectable if
	// it is stopped.
	periodic := j.Periodic != nil && j.Periodic.Enabled
	parameterized := j.IsParameterized()
	if periodic || parameterized {
		return j.Stop, nil
	}

	// If the job isn't dead it isn't eligible
	if j.Status != structs.JobStatusDead {
		return false, nil
	}

	// Any job that is stopped is eligible for garbage collection
	if j.Stop {
		return true, nil
	}

	switch j.Type {
	// Otherwise, batch and sysbatch jobs are eligible because they complete on
	// their own without a user stopping them.
	case structs.JobTypeBatch, structs.JobTypeSysBatch:
		return true, nil

	default:
		// other job types may not be GC until stopped
		return false, nil
	}
}

// jobIsPeriodic satisfies the ConditionalIndexFunc interface and creates an index
// on whether a job is periodic.
func jobIsPeriodic(obj interface{}) (bool, error) {
	j, ok := obj.(*structs.Job)
	if !ok {
		return false, fmt.Errorf("Unexpected type: %v", obj)
	}

	if j.Periodic != nil && j.Periodic.Enabled {
		return true, nil
	}

	return false, nil
}

// deploymentSchema returns the MemDB schema tracking a job's deployments
func deploymentSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "deployment",
		Indexes: map[string]*memdb.IndexSchema{
			// id index is used for direct lookup of an deployment by ID.
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},

			// create index is used for listing deploy, ordering them by
			// creation chronology. (Use a reverse iterator for newest first).
			//
			// There may be more than one deployment per CreateIndex.
			"create": {
				Name:         "create",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.UintFieldIndex{
							Field: "CreateIndex",
						},
						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
				},
			},

			// namespace is used to lookup evaluations by namespace.
			"namespace": {
				Name:         "namespace",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Namespace",
				},
			},

			// namespace_create index is used to lookup deployments by namespace
			// in their original chronological order based on CreateIndex.
			//
			// Use a prefix iterator (namespace_create_prefix) to iterate deployments
			// of a Namespace in order of CreateIndex.
			//
			// There may be more than one deployment per CreateIndex.
			"namespace_create": {
				Name:         "namespace_create",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					AllowMissing: false,
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},
						&memdb.UintFieldIndex{
							Field: "CreateIndex",
						},
						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
				},
			},

			// job index is used to lookup deployments by job
			"job": {
				Name:         "job",
				AllowMissing: false,
				Unique:       false,

				// Use a compound index so the tuple of (Namespace, JobID) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "JobID",
						},
					},
				},
			},
		},
	}
}

// periodicLaunchTableSchema returns the MemDB schema tracking the most recent
// launch time for a periodic job.
func periodicLaunchTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "periodic_launch",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is used for job management
			// and simple direct lookup. ID is required to be
			// unique.
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// Use a compound index so the tuple of (Namespace, JobID) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
				},
			},
		},
	}
}

// evalTableSchema returns the MemDB schema for the eval table.
// This table is used to store all the evaluations that are pending
// or recently completed.
func evalTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "evals",
		Indexes: map[string]*memdb.IndexSchema{
			// id index is used for direct lookup of an evaluation by ID.
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},

			// create index is used for listing evaluations, ordering them by
			// creation chronology. (Use a reverse iterator for newest first).
			"create": {
				Name:         "create",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.UintFieldIndex{
							Field: "CreateIndex",
						},
						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
				},
			},

			// job index is used to lookup evaluations by job ID.
			"job": {
				Name:         "job",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field:     "JobID",
							Lowercase: true,
						},

						&memdb.StringFieldIndex{
							Field:     "Status",
							Lowercase: true,
						},
					},
				},
			},

			// namespace is used to lookup evaluations by namespace.
			"namespace": {
				Name:         "namespace",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Namespace",
				},
			},

			// namespace_create index is used to lookup evaluations by namespace
			// in their original chronological order based on CreateIndex.
			//
			// Use a prefix iterator (namespace_prefix) on a Namespace to iterate
			// those evaluations in order of CreateIndex.
			"namespace_create": {
				Name:         "namespace_create",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					AllowMissing: false,
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},
						&memdb.UintFieldIndex{
							Field: "CreateIndex",
						},
						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
				},
			},
		},
	}
}

// allocTableSchema returns the MemDB schema for the allocation table.
// This table is used to store all the task allocations between task groups
// and nodes.
func allocTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "allocs",
		Indexes: map[string]*memdb.IndexSchema{
			// id index is used for direct lookup of allocation by ID.
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},

			// create index is used for listing allocations, ordering them by
			// creation chronology. (Use a reverse iterator for newest first).
			"create": {
				Name:         "create",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.UintFieldIndex{
							Field: "CreateIndex",
						},
						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
				},
			},

			// namespace is used to lookup evaluations by namespace.
			// todo(shoenig): i think we can deprecate this and other like it
			"namespace": {
				Name:         "namespace",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Namespace",
				},
			},

			// namespace_create index is used to lookup evaluations by namespace
			// in their original chronological order based on CreateIndex.
			//
			// Use a prefix iterator (namespace_prefix) on a Namespace to iterate
			// those evaluations in order of CreateIndex.
			"namespace_create": {
				Name:         "namespace_create",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					AllowMissing: false,
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},
						&memdb.UintFieldIndex{
							Field: "CreateIndex",
						},
						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
				},
			},

			// Node index is used to lookup allocations by node
			"node": {
				Name:         "node",
				AllowMissing: true, // Missing is allow for failed allocations
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "NodeID",
							Lowercase: true,
						},

						// Conditional indexer on if allocation is terminal
						&memdb.ConditionalIndex{
							Conditional: func(obj interface{}) (bool, error) {
								// Cast to allocation
								alloc, ok := obj.(*structs.Allocation)
								if !ok {
									return false, fmt.Errorf("wrong type, got %t should be Allocation", obj)
								}

								// Check if the allocation is terminal
								return alloc.TerminalStatus(), nil
							},
						},
					},
				},
			},

			// Job index is used to lookup allocations by job
			"job": {
				Name:         "job",
				AllowMissing: false,
				Unique:       false,

				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "JobID",
						},
					},
				},
			},

			// Eval index is used to lookup allocations by eval
			"eval": {
				Name:         "eval",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "EvalID",
				},
			},

			// Deployment index is used to lookup allocations by deployment
			"deployment": {
				Name:         "deployment",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "DeploymentID",
				},
			},

			// signing_key index is used to lookup live allocations by signing
			// key ID
			indexSigningKey: {
				Name:         indexSigningKey,
				AllowMissing: true, // terminal allocations won't be indexed
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "SigningKeyID",
						},
						&memdb.ConditionalIndex{
							Conditional: func(obj interface{}) (bool, error) {
								alloc, ok := obj.(*structs.Allocation)
								if !ok {
									return false, fmt.Errorf(
										"wrong type, got %t should be Allocation", obj)
								}
								// note: this isn't alloc.TerminalStatus(),
								// because we only want to consider the key
								// unused if the allocation is terminal on both
								// server and client
								return !(alloc.ClientTerminalStatus() && alloc.ServerTerminalStatus()), nil
							},
						},
					},
				},
			},
		},
	}
}

// vaultAccessorTableSchema returns the MemDB schema for the Vault Accessor
// Table. This table tracks Vault accessors for tokens created on behalf of
// allocations required Vault tokens.
func vaultAccessorTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "vault_accessors",
		Indexes: map[string]*memdb.IndexSchema{
			// The primary index is the accessor id
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Accessor",
				},
			},

			"alloc_id": {
				Name:         "alloc_id",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "AllocID",
				},
			},

			"node_id": {
				Name:         "node_id",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "NodeID",
				},
			},
		},
	}
}

// siTokenAccessorTableSchema returns the MemDB schema for the Service Identity
// token accessor table. This table tracks accessors for tokens created on behalf
// of allocations with Consul connect enabled tasks that need SI tokens.
func siTokenAccessorTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: siTokenAccessorTable,
		Indexes: map[string]*memdb.IndexSchema{
			// The primary index is the accessor id
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "AccessorID",
				},
			},

			"alloc_id": {
				Name:         "alloc_id",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "AllocID",
				},
			},

			"node_id": {
				Name:         "node_id",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "NodeID",
				},
			},
		},
	}
}

// aclPolicyTableSchema returns the MemDB schema for the policy table.
// This table is used to store the policies which are referenced by tokens
func aclPolicyTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl_policy",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Name",
				},
			},
			"job": {
				Name:         "job",
				AllowMissing: true,
				Unique:       false,
				Indexer:      &ACLPolicyJobACLFieldIndex{},
			},
		},
	}
}

// ACLPolicyJobACLFieldIndex is used to extract the policy's JobACL field and
// build an index on it.
type ACLPolicyJobACLFieldIndex struct{}

// FromObject is used to extract an index value from an
// object or to indicate that the index value is missing.
func (a *ACLPolicyJobACLFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	policy, ok := obj.(*structs.ACLPolicy)
	if !ok {
		return false, nil, fmt.Errorf("object %#v is not an ACLPolicy", obj)
	}

	if policy.JobACL == nil {
		return false, nil, nil
	}

	ns := policy.JobACL.Namespace
	if ns == "" {
		return false, nil, nil
	}
	jobID := policy.JobACL.JobID
	if jobID == "" {
		return false, nil, fmt.Errorf(
			"object %#v is not a valid ACLPolicy: Namespace without JobID", obj)
	}

	val := ns + "\x00" + jobID + "\x00"
	return true, []byte(val), nil
}

// FromArgs is used to build an exact index lookup based on arguments
func (a *ACLPolicyJobACLFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("must provide two arguments")
	}
	arg0, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	arg1, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}

	// Add the null character as a terminator
	arg0 += "\x00" + arg1 + "\x00"
	return []byte(arg0), nil
}

// PrefixFromArgs returns a prefix that should be used for scanning based on the arguments
func (a *ACLPolicyJobACLFieldIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := a.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// aclTokenTableSchema returns the MemDB schema for the tokens table.
// This table is used to store the bearer tokens which are used to authenticate
func aclTokenTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "acl_token",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "AccessorID",
				},
			},
			"create": {
				Name:         "create",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.UintFieldIndex{
							Field: "CreateIndex",
						},
						&memdb.StringFieldIndex{
							Field: "AccessorID",
						},
					},
				},
			},
			"secret": {
				Name:         "secret",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "SecretID",
				},
			},
			"global": {
				Name:         "global",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.FieldSetIndex{
					Field: "Global",
				},
			},
			indexExpiresGlobal: {
				Name:         indexExpiresGlobal,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexer.SingleIndexer{
					ReadIndex:  indexer.ReadIndex(indexer.IndexFromTimeQuery),
					WriteIndex: indexer.WriteIndex(indexExpiresGlobalFromACLToken),
				},
			},
			indexExpiresLocal: {
				Name:         indexExpiresLocal,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexer.SingleIndexer{
					ReadIndex:  indexer.ReadIndex(indexer.IndexFromTimeQuery),
					WriteIndex: indexer.WriteIndex(indexExpiresLocalFromACLToken),
				},
			},
		},
	}
}

func indexExpiresLocalFromACLToken(raw interface{}) ([]byte, error) {
	return indexExpiresFromACLToken(raw, false)
}

func indexExpiresGlobalFromACLToken(raw interface{}) ([]byte, error) {
	return indexExpiresFromACLToken(raw, true)
}

// indexExpiresFromACLToken implements the indexer.WriteIndex interface and
// allows us to use an ACL tokens ExpirationTime as an index, if it is a
// non-default value. This allows for efficient lookups when trying to deal
// with removal of expired tokens from state.
func indexExpiresFromACLToken(raw interface{}, global bool) ([]byte, error) {
	p, ok := raw.(*structs.ACLToken)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.ACLToken index", raw)
	}
	if p.Global != global {
		return nil, indexer.ErrMissingValueForIndex
	}
	if !p.HasExpirationTime() {
		return nil, indexer.ErrMissingValueForIndex
	}
	if p.ExpirationTime.Unix() < 0 {
		return nil, fmt.Errorf("token expiration time cannot be before the unix epoch: %s", p.ExpirationTime)
	}

	var b indexer.IndexBuilder
	b.Time(*p.ExpirationTime)
	return b.Bytes(), nil
}

// oneTimeTokenTableSchema returns the MemDB schema for the tokens table.
// This table is used to store one-time tokens for ACL tokens
func oneTimeTokenTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "one_time_token",
		Indexes: map[string]*memdb.IndexSchema{
			"secret": {
				Name:         "secret",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "OneTimeSecretID",
				},
			},
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "AccessorID",
				},
			},
		},
	}
}

// singletonRecord can be used to describe tables which should contain only 1 entry.
// Example uses include storing node config or cluster metadata blobs.
var singletonRecord = &memdb.ConditionalIndex{
	Conditional: func(interface{}) (bool, error) { return true, nil },
}

// schedulerConfigTableSchema returns the MemDB schema for the scheduler config table.
// This table is used to store configuration options for the scheduler
func schedulerConfigTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "scheduler_config",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: true,
				Unique:       true,
				Indexer:      singletonRecord, // we store only 1 scheduler config
			},
		},
	}
}

// clusterMetaTableSchema returns the MemDB schema for the scheduler config table.
func clusterMetaTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "cluster_meta",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer:      singletonRecord, // we store only 1 cluster metadata
			},
		},
	}
}

// CSIVolumes are identified by id globally, and searchable by driver
func csiVolumeTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "csi_volumes",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},
						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
				},
			},
			"plugin_id": {
				Name:         "plugin_id",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "PluginID",
				},
			},
		},
	}
}

// CSIPlugins are identified by id globally, and searchable by driver
func csiPluginTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "csi_plugins",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "ID",
				},
			},
		},
	}
}

// ScalingPolicyTargetFieldIndex is used to extract a field from an object
// using reflection and builds an index on that field.
type ScalingPolicyTargetFieldIndex struct {
	Field string

	// AllowMissing controls if the field should be ignored if the field is
	// not provided.
	AllowMissing bool
}

// FromObject is used to extract an index value from an
// object or to indicate that the index value is missing.
func (s *ScalingPolicyTargetFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	policy, ok := obj.(*structs.ScalingPolicy)
	if !ok {
		return false, nil, fmt.Errorf("object %#v is not a ScalingPolicy", obj)
	}

	if policy.Target == nil {
		return false, nil, nil
	}

	val, ok := policy.Target[s.Field]
	if !ok && !s.AllowMissing {
		return false, nil, nil
	}

	// Add the null character as a terminator
	val += "\x00"
	return true, []byte(val), nil
}

// FromArgs is used to build an exact index lookup based on arguments
func (s *ScalingPolicyTargetFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

// PrefixFromArgs returns a prefix that should be used for scanning based on the arguments
func (s *ScalingPolicyTargetFieldIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := s.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// scalingPolicyTableSchema returns the MemDB schema for the policy table.
func scalingPolicyTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "scaling_policy",
		Indexes: map[string]*memdb.IndexSchema{
			// Primary index is used for simple direct lookup.
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// UUID is uniquely identifying
				Indexer: &memdb.StringFieldIndex{
					Field: "ID",
				},
			},
			// Target index is used for listing by namespace or job, or looking up a specific target.
			// A given task group can have only a single scaling policies, so this is guaranteed to be unique.
			"target": {
				Name:   "target",
				Unique: false,

				// Use a compound index so the tuple of (Namespace, Job, Group, Task) is
				// used when looking for a policy
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&ScalingPolicyTargetFieldIndex{
							Field:        "Namespace",
							AllowMissing: true,
						},

						&ScalingPolicyTargetFieldIndex{
							Field:        "Job",
							AllowMissing: true,
						},

						&ScalingPolicyTargetFieldIndex{
							Field:        "Group",
							AllowMissing: true,
						},

						&ScalingPolicyTargetFieldIndex{
							Field:        "Task",
							AllowMissing: true,
						},
					},
				},
			},
			// Type index is used for listing by policy type
			"type": {
				Name:         "type",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Type",
				},
			},
			// Used to filter by enabled
			"enabled": {
				Name:         "enabled",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.FieldSetIndex{
					Field: "Enabled",
				},
			},
		},
	}
}

// scalingEventTableSchema returns the memdb schema for job scaling events
func scalingEventTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "scaling_event",
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,

				// Use a compound index so the tuple of (Namespace, JobID) is
				// uniquely identifying
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},

						&memdb.StringFieldIndex{
							Field: "JobID",
						},
					},
				},
			},
		},
	}
}

// namespaceTableSchema returns the MemDB schema for the namespace table.
func namespaceTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableNamespaces,
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Name",
				},
			},
			"quota": {
				Name:         "quota",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Quota",
				},
			},
		},
	}
}

// serviceRegistrationsTableSchema returns the MemDB schema for Nomad native
// service registrations.
func serviceRegistrationsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableServiceRegistrations,
		Indexes: map[string]*memdb.IndexSchema{
			// The serviceID in combination with namespace forms a unique
			// identifier for a service registration. This is used to look up
			// and delete services in individual isolation.
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},
						&memdb.StringFieldIndex{
							Field: "ID",
						},
					},
				},
			},
			indexServiceName: {
				Name:         indexServiceName,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},
						&memdb.StringFieldIndex{
							Field: "ServiceName",
						},
					},
				},
			},
			indexJob: {
				Name:         indexJob,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},
						&memdb.StringFieldIndex{
							Field: "JobID",
						},
					},
				},
			},
			// The nodeID index allows lookups and deletions to be performed
			// for an entire node. This is primarily used when a node becomes
			// lost.
			indexNodeID: {
				Name:         indexNodeID,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "NodeID",
				},
			},
			indexAllocID: {
				Name:         indexAllocID,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "AllocID",
				},
			},
		},
	}
}

// variablesTableSchema returns the MemDB schema for Nomad variables.
func variablesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableVariables,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Namespace",
						},
						&memdb.StringFieldIndex{
							Field: "Path",
						},
					},
				},
			},
			indexKeyID: {
				Name:         indexKeyID,
				AllowMissing: false,
				Indexer:      &variableKeyIDFieldIndexer{},
			},
			indexPath: {
				Name:         indexPath,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "Path",
				},
			},
		},
	}
}

type variableKeyIDFieldIndexer struct{}

// FromArgs implements go-memdb/Indexer and is used to build an exact
// index lookup based on arguments
func (s *variableKeyIDFieldIndexer) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

// PrefixFromArgs implements go-memdb/PrefixIndexer and returns a
// prefix that should be used for scanning based on the arguments
func (s *variableKeyIDFieldIndexer) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := s.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// FromObject implements go-memdb/SingleIndexer and is used to extract
// an index value from an object or to indicate that the index value
// is missing.
func (s *variableKeyIDFieldIndexer) FromObject(obj interface{}) (bool, []byte, error) {
	variable, ok := obj.(*structs.VariableEncrypted)
	if !ok {
		return false, nil, fmt.Errorf("object %#v is not a Variable", obj)
	}

	keyID := variable.KeyID
	if keyID == "" {
		return false, nil, nil
	}

	// Add the null character as a terminator
	keyID += "\x00"
	return true, []byte(keyID), nil
}

// variablesQuotasTableSchema returns the MemDB schema for Nomad variables
// quotas tracking
func variablesQuotasTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableVariablesQuotas,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Namespace",
					Lowercase: true,
				},
			},
		},
	}
}

// wrappedRootKeySchema returns the MemDB schema for wrapped Nomad root keys
func wrappedRootKeySchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableRootKeys,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field:     "KeyID",
					Lowercase: true,
				},
			},
		},
	}
}

func aclRolesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableACLRoles,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "ID",
				},
			},
			indexName: {
				Name:         indexName,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Name",
				},
			},
		},
	}
}

func aclAuthMethodsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableACLAuthMethods,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Name",
				},
			},
		},
	}
}

func bindingRulesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: TableACLBindingRules,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "ID",
				},
			},
			indexAuthMethod: {
				Name:         indexAuthMethod,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field: "AuthMethod",
				},
			},
		},
	}
}
