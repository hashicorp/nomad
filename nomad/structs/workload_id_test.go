// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"strings"
	"testing"
	"time"

	jwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestNewIdentityClaims(t *testing.T) {
	ci.Parallel(t)

	job := &Job{
		ID:        "job",
		ParentID:  "parentJob",
		Name:      "job",
		Namespace: "default",
		Region:    "global",

		TaskGroups: []*TaskGroup{
			{
				Name: "group",
				Services: []*Service{{
					Name:      "group-service-",
					PortLabel: "http",
					Identity: &WorkloadIdentity{
						Audience: []string{"group-service.consul.io"},
					},
				}},
				Tasks: []*Task{
					{
						Name: "task",
						Identity: &WorkloadIdentity{
							Name:     "default-identity",
							Audience: []string{"example.com"},
						},
						Identities: []*WorkloadIdentity{
							{
								Name:     "alt-identity",
								Audience: []string{"alt.example.com"},
							},
							{
								Name:     "consul_default",
								Audience: []string{"consul.io"},
							},
							{
								Name:     "vault_default",
								Audience: []string{"vault.io"},
							},
						},
						Services: []*Service{{
							Name:      "task-service",
							PortLabel: "http",
							Identity: &WorkloadIdentity{
								Audience: []string{"task-service.consul.io"},
							},
						}},
					},
					{
						Name: "consul-vault-task",
						Consul: &Consul{
							Namespace: "task-consul-namespace",
						},
						Vault: &Vault{
							Namespace: "vault-namespace",
							Role:      "role-from-spec-group",
						},
						Identity: &WorkloadIdentity{
							Name:     "default-identity",
							Audience: []string{"example.com"},
						},
						Identities: []*WorkloadIdentity{
							{
								Name:     "consul_default",
								Audience: []string{"consul.io"},
							},
							{
								Name:     "vault_default",
								Audience: []string{"vault.io"},
							},
						},
						Services: []*Service{{
							Name:      "consul-task-service",
							PortLabel: "http",
							Identity: &WorkloadIdentity{
								Audience: []string{"task-service.consul.io"},
							},
						}},
					},
				},
			},
			{
				Name: "consul-group",
				Consul: &Consul{
					Namespace: "group-consul-namespace",
				},
				Services: []*Service{{
					Name:      "group-service",
					PortLabel: "http",
					Identity: &WorkloadIdentity{
						Audience: []string{"group-service.consul.io"},
					},
				}},
				Tasks: []*Task{
					{
						Name: "task",
						Identity: &WorkloadIdentity{
							Name:     "default-identity",
							Audience: []string{"example.com"},
						},
						Identities: []*WorkloadIdentity{
							{
								Name:     "alt-identity",
								Audience: []string{"alt.example.com"},
							},
							{
								Name:     "consul_default",
								Audience: []string{"consul.io"},
							},
							{
								Name:     "vault_default",
								Audience: []string{"vault.io"},
							},
						},
						Services: []*Service{{
							Name:      "task-service",
							PortLabel: "http",
							Identity: &WorkloadIdentity{
								Audience: []string{"task-service.consul.io"},
							},
						}},
					},
					{
						Name: "consul-vault-task",
						Consul: &Consul{
							Namespace: "task-consul-namespace",
						},
						Vault: &Vault{
							Namespace: "vault-namespace",
							Role:      "role-from-spec-consul-group",
						},
						Identity: &WorkloadIdentity{
							Name:     "default-identity",
							Audience: []string{"example.com"},
						},
						Identities: []*WorkloadIdentity{
							{
								Name:     "consul_default",
								Audience: []string{"consul.io"},
							},
							{
								Name:     "vault_default",
								Audience: []string{"vault.io"},
							},
						},
						Services: []*Service{{
							Name:      "consul-task-service",
							PortLabel: "http",
							Identity: &WorkloadIdentity{
								Audience: []string{"consul.io"},
							},
						}},
					},
				},
			},
		},
	}
	job.Canonicalize()

	expectedClaims := map[string]*IdentityClaims{
		// group: no consul.
		"job/group/services/group-service": {
			Namespace:   "default",
			JobID:       "parentJob",
			ServiceName: "group-service",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:group:group-service:consul-service_group-service-http",
				Audience: jwt.Audience{"group-service.consul.io"},
			},
			ExtraClaims: map[string]string{},
		},
		// group: no consul.
		// task:  no consul, no vault.
		"job/group/task/default-identity": {
			Namespace: "default",
			JobID:     "parentJob",
			TaskName:  "task",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:group:task:default-identity",
				Audience: jwt.Audience{"example.com"},
			},
			ExtraClaims: map[string]string{},
		},
		"job/group/task/alt-identity": {
			Namespace: "default",
			JobID:     "parentJob",
			TaskName:  "task",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:group:task:alt-identity",
				Audience: jwt.Audience{"alt.example.com"},
			},
			ExtraClaims: map[string]string{},
		},
		// No ConsulNamespace because there is no consul block at either task
		// or group level.
		"job/group/task/consul_default": {
			ConsulNamespace: "",
			Namespace:       "default",
			JobID:           "parentJob",
			TaskName:        "task",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:group:task:consul_default",
				Audience: jwt.Audience{"consul.io"},
			},
			ExtraClaims: map[string]string{},
		},
		// No VaultNamespace because there is no vault block at either task
		// or group level.
		"job/group/task/vault_default": {
			VaultNamespace: "",
			Namespace:      "default",
			JobID:          "parentJob",
			TaskName:       "task",
			VaultRole:      "", // not specified in jobspec
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:group:task:vault_default",
				Audience: jwt.Audience{"vault.io"},
			},
			ExtraClaims: map[string]string{
				"nomad_workload_id": "global:default:parentJob",
			},
		},
		"job/group/task/services/task-service": {
			Namespace:   "default",
			JobID:       "parentJob",
			ServiceName: "task-service",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:group:task-service:consul-service_task-task-service-http",
				Audience: jwt.Audience{"task-service.consul.io"},
			},
			ExtraClaims: map[string]string{},
		},
		// group: no consul.
		// task:  with consul, with vault.
		"job/group/consul-vault-task/default-identity": {
			Namespace: "default",
			JobID:     "parentJob",
			TaskName:  "consul-vault-task",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:group:consul-vault-task:default-identity",
				Audience: jwt.Audience{"example.com"},
			},
			ExtraClaims: map[string]string{},
		},
		// Use task-level Consul namespace.
		"job/group/consul-vault-task/consul_default": {
			ConsulNamespace: "task-consul-namespace",
			Namespace:       "default",
			JobID:           "parentJob",
			TaskName:        "consul-vault-task",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:group:consul-vault-task:consul_default",
				Audience: jwt.Audience{"consul.io"},
			},
			ExtraClaims: map[string]string{},
		},
		// Use task-level Vault namespace.
		"job/group/consul-vault-task/vault_default": {
			VaultNamespace: "vault-namespace",
			Namespace:      "default",
			JobID:          "parentJob",
			TaskName:       "consul-vault-task",
			VaultRole:      "role-from-spec-group",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:group:consul-vault-task:vault_default",
				Audience: jwt.Audience{"vault.io"},
			},
			ExtraClaims: map[string]string{
				"nomad_workload_id": "global:default:parentJob",
			},
		},
		// Use task-level Consul namespace for task services.
		"job/group/consul-vault-task/services/consul-vault-task-service": {
			ConsulNamespace: "task-consul-namespace",
			Namespace:       "default",
			JobID:           "parentJob",
			ServiceName:     "consul-vault-task-service",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:group:consul-vault-task-service:consul-service_consul-vault-task-service-http",
				Audience: jwt.Audience{"consul.io"},
			},
			ExtraClaims: map[string]string{},
		},
		// group: with consul.
		// Use group-level Consul namespace for group services.
		"job/consul-group/services/group-service": {
			ConsulNamespace: "group-consul-namespace",
			Namespace:       "default",
			JobID:           "parentJob",
			ServiceName:     "group-service",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:consul-group:group-service:consul-service_group-service-http",
				Audience: jwt.Audience{"group-service.consul.io"},
			},
			ExtraClaims: map[string]string{},
		},
		// group: with consul.
		// task:  no consul, no vault.
		"job/consul-group/task/default-identity": {
			Namespace: "default",
			JobID:     "parentJob",
			TaskName:  "task",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:consul-group:task:default-identity",
				Audience: jwt.Audience{"example.com"},
			},
			ExtraClaims: map[string]string{},
		},
		"job/consul-group/task/alt-identity": {
			Namespace: "default",
			JobID:     "parentJob",
			TaskName:  "task",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:consul-group:task:alt-identity",
				Audience: jwt.Audience{"alt.example.com"},
			},
			ExtraClaims: map[string]string{},
		},
		// Use group-level Consul namespace because task doesn't have a consul
		// block.
		"job/consul-group/task/consul_default": {
			ConsulNamespace: "group-consul-namespace",
			Namespace:       "default",
			JobID:           "parentJob",
			TaskName:        "task",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:consul-group:task:consul_default",
				Audience: jwt.Audience{"consul.io"},
			},
			ExtraClaims: map[string]string{},
		},
		"job/consul-group/task/vault_default": {
			Namespace: "default",
			JobID:     "parentJob",
			TaskName:  "task",
			VaultRole: "", // not specified in jobspec
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:consul-group:task:vault_default",
				Audience: jwt.Audience{"vault.io"},
			},
			ExtraClaims: map[string]string{
				"nomad_workload_id": "global:default:parentJob",
			},
		},
		// Use group-level Consul namespace for task service because task
		// doesn't have a consul block.
		"job/consul-group/task/services/task-service": {
			ConsulNamespace: "group-consul-namespace",
			Namespace:       "default",
			JobID:           "parentJob",
			ServiceName:     "task-service",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:consul-group:task-service:consul-service_task-task-service-http",
				Audience: jwt.Audience{"task-service.consul.io"},
			},
			ExtraClaims: map[string]string{},
		},
		// group: no consul.
		// task:  with consul, with vault.
		"job/consul-group/consul-vault-task/default-identity": {
			Namespace: "default",
			JobID:     "parentJob",
			TaskName:  "consul-vault-task",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:consul-group:consul-vault-task:default-identity",
				Audience: jwt.Audience{"example.com"},
			},
			ExtraClaims: map[string]string{},
		},
		// Use task-level Consul namespace.
		"job/consul-group/consul-vault-task/consul_default": {
			ConsulNamespace: "task-consul-namespace",
			Namespace:       "default",
			JobID:           "parentJob",
			TaskName:        "consul-vault-task",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:consul-group:consul-vault-task:consul_default",
				Audience: jwt.Audience{"consul.io"},
			},
			ExtraClaims: map[string]string{},
		},
		"job/consul-group/consul-vault-task/vault_default": {
			VaultNamespace: "vault-namespace",
			Namespace:      "default",
			JobID:          "parentJob",
			TaskName:       "consul-vault-task",
			VaultRole:      "role-from-spec-consul-group",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:consul-group:consul-vault-task:vault_default",
				Audience: jwt.Audience{"vault.io"},
			},
			ExtraClaims: map[string]string{
				"nomad_workload_id": "global:default:parentJob",
			},
		},
		// Use task-level Consul namespace for task services.
		"job/consul-group/consul-vault-task/services/consul-task-service": {
			ConsulNamespace: "task-consul-namespace",
			Namespace:       "default",
			JobID:           "parentJob",
			ServiceName:     "consul-task-service",
			Claims: jwt.Claims{
				Subject:  "global:default:parentJob:consul-group:consul-task-service:consul-service_consul-vault-task-consul-task-service-http",
				Audience: jwt.Audience{"consul.io"},
			},
			ExtraClaims: map[string]string{},
		},
	}

	// Generate service identity names.
	for _, tg := range job.TaskGroups {
		for _, s := range tg.Services {
			if s.Identity != nil {
				s.Identity.Name = s.MakeUniqueIdentityName()
			}
		}
		for _, t := range tg.Tasks {
			for _, s := range t.Services {
				if s.Identity != nil {
					s.Identity.Name = s.MakeUniqueIdentityName()
				}
			}
		}
	}

	// Find all indentites in test job and create a test case for each.
	// Tests for identities missing from expectedClaims are skipped.
	type testCase struct {
		name           string
		group          string
		task           *Task
		svc            *Service
		wid            *WorkloadIdentity
		wiHandle       *WIHandle
		expectedClaims *IdentityClaims
	}
	testCases := []testCase{}
	for _, tg := range job.TaskGroups {
		path := job.ID + "/" + tg.Name

		for _, s := range tg.Services {
			path := path + "/services/" + s.Name

			testCases = append(testCases, testCase{
				name:           path,
				group:          tg.Name,
				svc:            s,
				wid:            s.Identity,
				wiHandle:       s.IdentityHandle(nil),
				expectedClaims: expectedClaims[path],
			})
		}

		for _, t := range tg.Tasks {
			path := path + "/" + t.Name

			for _, wid := range append(t.Identities, t.Identity) {
				if wid == nil {
					continue
				}

				path := path + "/" + wid.Name
				testCases = append(testCases, testCase{
					name:           path,
					group:          tg.Name,
					task:           t,
					wid:            wid,
					wiHandle:       t.IdentityHandle(wid),
					expectedClaims: expectedClaims[path],
				})
			}

			for _, s := range t.Services {
				path := path + "/services/" + s.Name
				testCases = append(testCases, testCase{
					name:           path,
					group:          tg.Name,
					task:           t,
					svc:            s,
					wid:            s.Identity,
					wiHandle:       s.IdentityHandle(nil),
					expectedClaims: expectedClaims[path],
				})
			}
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedClaims == nil {
				t.Skip("missing expected claims")
			}

			now := time.Now()
			alloc := &Allocation{
				ID:        uuid.Generate(),
				Namespace: job.Namespace,
				JobID:     job.ID,
				TaskGroup: tc.group,
			}

			got := NewIdentityClaimsBuilder(job, alloc, tc.wiHandle, tc.wid).
				WithTask(tc.task).
				WithService(tc.svc).
				WithConsul().
				WithVault(map[string]string{
					"nomad_workload_id": "${job.region}:${job.namespace}:${job.id}",
				}).
				Build(now)

			must.Eq(t, tc.expectedClaims, got, must.Cmp(cmpopts.IgnoreFields(
				IdentityClaims{},
				"ID", "AllocationID", "IssuedAt", "NotBefore",
			)))
			must.Eq(t, alloc.ID, got.AllocationID)
			must.Eq(t, jwt.NewNumericDate(now), got.IssuedAt)
			must.Eq(t, jwt.NewNumericDate(now), got.NotBefore)
		})
	}
}

func TestWorkloadIdentity_Equal(t *testing.T) {
	ci.Parallel(t)

	var orig *WorkloadIdentity

	newWI := orig.Copy()
	must.Equal(t, orig, newWI)

	orig = &WorkloadIdentity{}
	must.NotEqual(t, orig, newWI)

	newWI = &WorkloadIdentity{}
	must.Equal(t, orig, newWI)

	orig.ChangeMode = WIChangeModeSignal
	must.NotEqual(t, orig, newWI)

	orig.ChangeMode = ""
	must.Equal(t, orig, newWI)

	orig.ChangeSignal = "SIGHUP"
	must.NotEqual(t, orig, newWI)

	orig.ChangeSignal = ""
	must.Equal(t, orig, newWI)

	orig.Env = true
	must.NotEqual(t, orig, newWI)

	newWI.Env = true
	must.Equal(t, orig, newWI)

	newWI.File = true
	must.NotEqual(t, orig, newWI)

	newWI.File = false
	must.Equal(t, orig, newWI)

	newWI.Filepath = "foo"
	must.NotEqual(t, orig, newWI)

	newWI.Filepath = ""
	must.Equal(t, orig, newWI)

	newWI.Name = "foo"
	must.NotEqual(t, orig, newWI)

	newWI.Name = ""
	must.Equal(t, orig, newWI)

	newWI.Audience = []string{"foo"}
	must.NotEqual(t, orig, newWI)

	newWI.Audience = orig.Audience
	must.Equal(t, orig, newWI)

	newWI.TTL = 123 * time.Hour
	must.NotEqual(t, orig, newWI)
}

// TestWorkloadIdentity_Validate asserts that canonicalized workload identities
// validate and emit warnings as expected.
func TestWorkloadIdentity_Validate(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Desc string
		In   WorkloadIdentity
		Exp  WorkloadIdentity
		Err  string
		Warn string
	}{
		{
			Desc: "Empty",
			In:   WorkloadIdentity{},
			Exp: WorkloadIdentity{
				Name:     WorkloadIdentityDefaultName,
				Audience: []string{WorkloadIdentityDefaultAud},
			},
		},
		{
			Desc: "Default audience",
			In: WorkloadIdentity{
				Name: WorkloadIdentityDefaultName,
			},
			Exp: WorkloadIdentity{
				Name:     WorkloadIdentityDefaultName,
				Audience: []string{WorkloadIdentityDefaultAud},
			},
		},
		{
			Desc: "Ok",
			In: WorkloadIdentity{
				Name:       "foo-id",
				Audience:   []string{"http://nomadproject.io/"},
				ChangeMode: WIChangeModeRestart,
				Env:        true,
				File:       true,
				TTL:        time.Hour,
			},
			Exp: WorkloadIdentity{
				Name:       "foo-id",
				Audience:   []string{"http://nomadproject.io/"},
				ChangeMode: WIChangeModeRestart,
				Env:        true,
				File:       true,
				TTL:        time.Hour,
			},
		},
		{
			Desc: "OkSignal",
			In: WorkloadIdentity{
				Name:         "foo-id",
				Audience:     []string{"http://nomadproject.io/"},
				ChangeMode:   WIChangeModeSignal,
				ChangeSignal: "sighup",
				File:         true,
				TTL:          time.Hour,
			},
			Exp: WorkloadIdentity{
				Name:         "foo-id",
				Audience:     []string{"http://nomadproject.io/"},
				ChangeMode:   WIChangeModeSignal,
				ChangeSignal: "SIGHUP",
				File:         true,
				TTL:          time.Hour,
			},
		},
		{
			Desc: "Warn on env without restart",
			In: WorkloadIdentity{
				Name:     "foo-id",
				Audience: []string{"http://nomadproject.io/"},
				Env:      true,
				TTL:      time.Hour,
			},
			Exp: WorkloadIdentity{
				Name:     "foo-id",
				Audience: []string{"http://nomadproject.io/"},
				Env:      true,
				TTL:      time.Hour,
			},
			Warn: `using env=true without change_mode="restart" may result in task not getting updated identity`,
		},
		{
			Desc: "Signal without signal",
			In: WorkloadIdentity{
				Name:       "foo-id",
				Audience:   []string{"http://nomadproject.io/"},
				ChangeMode: WIChangeModeSignal,
				Env:        true,
				TTL:        time.Hour,
			},
			Err: `change_signal must be specified`,
		},
		{
			Desc: "Restart with signal",
			In: WorkloadIdentity{
				Name:         "foo-id",
				Audience:     []string{"http://nomadproject.io/"},
				ChangeMode:   WIChangeModeRestart,
				ChangeSignal: "SIGHUP",
				File:         true,
				TTL:          time.Hour,
			},
			Err: `can only use change_signal=`,
		},
		{
			Desc: "Be reasonable",
			In: WorkloadIdentity{
				Name: strings.Repeat("x", 1025),
			},
			Err: "invalid name",
		},
		{
			Desc: "No hacks",
			In: WorkloadIdentity{
				Name: "../etc/passwd",
			},
			Err: "invalid name",
		},
		{
			Desc: "No Windows hacks",
			In: WorkloadIdentity{
				Name: `A:\hacks`,
			},
			Err: "invalid name",
		},
		{
			Desc: "Empty audience",
			In: WorkloadIdentity{
				Name:     "foo",
				Audience: []string{"ok", ""},
			},
			Err: "an empty string is an invalid audience (2)",
		},
		{
			Desc: "Warn audience",
			In: WorkloadIdentity{
				Name: "foo",
			},
			Exp: WorkloadIdentity{
				Name: "foo",
			},
			Warn: "identities without an audience are insecure",
		},
		{
			Desc: "Warn too many audiences",
			In: WorkloadIdentity{
				Name:     "foo",
				Audience: []string{"foo", "bar"},
			},
			Exp: WorkloadIdentity{
				Name:     "foo",
				Audience: []string{"foo", "bar"},
			},
			Warn: "while multiple audiences is allowed, it is more secure to use 1 audience per identity",
		},
		{
			Desc: "Bad TTL",
			In: WorkloadIdentity{
				Name: "foo",
				TTL:  -1 * time.Hour,
			},
			Err: "ttl must be >= 0",
		},
		{
			Desc: "No TTL",
			In: WorkloadIdentity{
				Name:     "foo",
				Audience: []string{"foo"},
			},
			Exp: WorkloadIdentity{
				Name:     "foo",
				Audience: []string{"foo"},
			},
			Warn: "identities without an expiration are insecure",
		},
		{
			Desc: "Filepath set without file",
			In: WorkloadIdentity{
				Name:     "foo",
				Filepath: "foo",
			},
			Err: "file parameter must be true in order to specify filepath",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Desc, func(t *testing.T) {
			tc.In.Canonicalize()

			if err := tc.In.Validate(); err != nil {
				if tc.Err == "" {
					t.Fatalf("unexpected validation error: %s", err)
				}
				must.ErrorContains(t, err, tc.Err)
				return
			}

			// Only compare valid structs
			must.Eq(t, tc.Exp, tc.In)

			if err := tc.In.Warnings(); err != nil {
				if tc.Warn == "" {
					t.Fatalf("unexpected warnings: %s", err)
				}
				must.ErrorContains(t, err, tc.Warn)
				return
			}
		})
	}
}

func TestWorkloadIdentity_Nil(t *testing.T) {
	ci.Parallel(t)

	var nilWID *WorkloadIdentity

	nilWID = nilWID.Copy()
	must.Nil(t, nilWID)

	must.True(t, nilWID.Equal(nil))

	nilWID.Canonicalize()

	must.Error(t, nilWID.Validate())

	must.Error(t, nilWID.Warnings())
}
