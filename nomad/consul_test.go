// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

var _ ConsulConfigsAPI = (*consulConfigsAPI)(nil)

func TestConsulConfigsAPI_SetCE(t *testing.T) {
	ci.Parallel(t)

	try := func(t *testing.T,
		expectErr error,
		expectKey string,
		expectConfig api.ConfigEntry,
		expectWriteOpts *api.WriteOptions,
		f func(ConsulConfigsAPI) error) {

		logger := testlog.HCLogger(t)
		configsAPI := consul.NewMockConfigsAPI(logger)
		configsAPI.SetError(expectErr)
		configsAPIFunc := func(_ string) consul.ConfigAPI { return configsAPI }

		c := NewConsulConfigsAPI(configsAPIFunc, logger)
		err := f(c) // set the config entry

		entry, wo := configsAPI.GetEntry(expectKey)
		must.Eq(t, expectConfig, entry)
		must.Eq(t, expectWriteOpts, wo)

		switch expectErr {
		case nil:
			must.NoError(t, err)
		default:
			must.EqError(t, err, expectErr.Error())
		}
	}

	ctx := context.Background()

	// existing behavior is no set namespace
	consulNamespace := ""
	partition := "foo"

	ingressCE := new(structs.ConsulIngressConfigEntry)
	t.Run("ingress ok", func(t *testing.T) {
		try(t, nil, "ig",
			&api.IngressGatewayConfigEntry{Kind: "ingress-gateway", Name: "ig"},
			&api.WriteOptions{Partition: partition},
			func(c ConsulConfigsAPI) error {
				return c.SetIngressCE(
					ctx, consulNamespace, "ig", structs.ConsulDefaultCluster, partition, ingressCE)
			})
	})

	t.Run("ingress fail", func(t *testing.T) {
		try(t, errors.New("consul broke"),
			"ig", nil, nil,
			func(c ConsulConfigsAPI) error {
				return c.SetIngressCE(
					ctx, consulNamespace, "ig", structs.ConsulDefaultCluster, partition, ingressCE)
			})
	})

	terminatingCE := new(structs.ConsulTerminatingConfigEntry)
	t.Run("terminating ok", func(t *testing.T) {
		try(t, nil, "tg",
			&api.TerminatingGatewayConfigEntry{Kind: "terminating-gateway", Name: "tg"},
			&api.WriteOptions{Partition: partition},
			func(c ConsulConfigsAPI) error {
				return c.SetTerminatingCE(
					ctx, consulNamespace, "tg", structs.ConsulDefaultCluster, partition, terminatingCE)
			})
	})

	t.Run("terminating fail", func(t *testing.T) {
		try(t, errors.New("consul broke"),
			"tg", nil, nil,
			func(c ConsulConfigsAPI) error {
				return c.SetTerminatingCE(
					ctx, consulNamespace, "tg", structs.ConsulDefaultCluster, partition, terminatingCE)
			})
	})

	// also mesh
}
