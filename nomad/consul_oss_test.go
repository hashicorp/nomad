// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestConsulACLsAPI_CheckPermissions_oss(t *testing.T) {
	ci.Parallel(t)

	// In Nomad OSS, CheckPermissions will only receive "" as input for the
	// namespace parameter - as the ConsulUsage map from namespace to usages will
	// always contain one key - the empty string.

	try := func(t *testing.T, namespace string, usage *structs.ConsulUsage, secretID string, exp error) {
		logger := testlog.HCLogger(t)
		aclAPI := consul.NewMockACLsAPI(logger)
		cAPI := NewConsulACLsAPI(aclAPI, logger, nil)

		err := cAPI.CheckPermissions(context.Background(), namespace, usage, secretID)
		if exp == nil {
			require.NoError(t, err)
		} else {
			require.Equal(t, exp.Error(), err.Error())
		}
	}

	t.Run("check-permissions kv read", func(t *testing.T) {
		t.Run("uses kv has permission", func(t *testing.T) {
			u := &structs.ConsulUsage{KV: true}
			try(t, "", u, consul.ExampleOperatorTokenID5, nil)
		})

		t.Run("uses kv without permission", func(t *testing.T) {
			u := &structs.ConsulUsage{KV: true}
			try(t, "", u, consul.ExampleOperatorTokenID1, errors.New("insufficient Consul ACL permissions to use template"))
		})

		t.Run("uses kv no token", func(t *testing.T) {
			u := &structs.ConsulUsage{KV: true}
			try(t, "", u, "", errors.New("missing consul token"))
		})

		t.Run("uses kv nonsense token", func(t *testing.T) {
			u := &structs.ConsulUsage{KV: true}
			try(t, "", u, "47d33e22-720a-7fe6-7d7f-418bf844a0be", errors.New("unable to read consul token: no such token"))
		})

		t.Run("no kv no token", func(t *testing.T) {
			u := &structs.ConsulUsage{KV: false}
			try(t, "", u, "", nil)
		})
	})

	t.Run("check-permissions service write", func(t *testing.T) {
		usage := &structs.ConsulUsage{Services: []string{"service1"}}

		t.Run("operator has service write", func(t *testing.T) {
			try(t, "", usage, consul.ExampleOperatorTokenID1, nil)
		})

		t.Run("operator has service_prefix write", func(t *testing.T) {
			u := &structs.ConsulUsage{Services: []string{"foo-service1"}}
			try(t, "", u, consul.ExampleOperatorTokenID2, nil)
		})

		t.Run("operator has service_prefix write wrong prefix", func(t *testing.T) {
			u := &structs.ConsulUsage{Services: []string{"bar-service1"}}
			try(t, "", u, consul.ExampleOperatorTokenID2, errors.New(`insufficient Consul ACL permissions to write service "bar-service1"`))
		})

		t.Run("operator permissions insufficient", func(t *testing.T) {
			try(t, "", usage, consul.ExampleOperatorTokenID3, errors.New(`insufficient Consul ACL permissions to write service "service1"`))
		})

		t.Run("operator provided no token", func(t *testing.T) {
			try(t, "", usage, "", errors.New("missing consul token"))
		})

		t.Run("operator provided nonsense token", func(t *testing.T) {
			try(t, "", usage, "f1682bde-1e71-90b1-9204-85d35467ba61", errors.New("unable to read consul token: no such token"))
		})
	})
}
