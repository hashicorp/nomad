// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package allocrunner

import (
	"errors"
	"slices"
	"testing"

	"github.com/coreos/go-iptables/iptables"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestNewIPTables(t *testing.T) {
	for family, expect := range map[structs.NodeNetworkAF]iptables.Protocol{
		structs.NodeNetworkAF_IPv6: iptables.ProtocolIPv6,
		structs.NodeNetworkAF_IPv4: iptables.ProtocolIPv4,
		"other":                    iptables.ProtocolIPv4,
	} {
		t.Run(string(family), func(t *testing.T) {
			mgr, err := newIPTables(family)
			must.NoError(t, err)
			cast := mgr.(*iptables.IPTables)
			must.Eq(t, expect, cast.Proto(), must.Sprint("unexpected ip family"))

			cleanup, err := newIPTablesCleanup(family)
			must.NoError(t, err)
			cast = cleanup.(*iptables.IPTables)
			must.Eq(t, expect, cast.Proto(), must.Sprint("unexpected ip family"))

			chain, err := newIPTablesChain(family)
			must.NoError(t, err)
			cast = chain.(*iptables.IPTables)
			must.Eq(t, expect, cast.Proto(), must.Sprint("unexpected ip family"))
		})
	}
}

func TestIPTables_ensureChainRule(t *testing.T) {
	ipt := &mockIPTablesChain{}
	err := ensureChainRule(ipt, "test-bridge", "1.1.1.1/1")
	must.NoError(t, err)
	must.Eq(t, ipt.chain, cniAdminChainName)
	must.Eq(t, ipt.table, "filter")
	must.Eq(t, ipt.rules, []string{"-o", "test-bridge", "-d", "1.1.1.1/1", "-j", "ACCEPT"})
}

type mockIPTablesCleanup struct {
	listCall  [2]string
	listRules []string
	listErr   error

	deleteCall [2]string
	deleteErr  error

	clearCall [2]string
	clearErr  error
}

func (ipt *mockIPTablesCleanup) List(table, chain string) ([]string, error) {
	ipt.listCall[0], ipt.listCall[1] = table, chain
	return ipt.listRules, ipt.listErr
}

func (ipt *mockIPTablesCleanup) Delete(table, chain string, rule ...string) error {
	ipt.deleteCall[0], ipt.deleteCall[1] = table, chain
	return ipt.deleteErr
}

func (ipt *mockIPTablesCleanup) ClearAndDeleteChain(table, chain string) error {
	ipt.clearCall[0], ipt.clearCall[1] = table, chain
	return ipt.clearErr
}

type mockIPTablesChain struct {
	// we're not keeping a complete database of iptables hierarchy,
	// just the one table-chain-rules combo we expect to create.
	table string
	chain string
	rules []string

	// we'll error if NewChain or Append are called more than once
	newChainCalled bool
	appendCalled   bool

	listChainsErr error
	newChainErr   error
	existsErr     error
	appendErr     error
}

func (ipt *mockIPTablesChain) ListChains(table string) ([]string, error) {
	return []string{ipt.chain}, ipt.listChainsErr
}

func (ipt *mockIPTablesChain) NewChain(table string, chain string) error {
	if ipt.newChainCalled {
		return errors.New("ipt.NewChain should only be called once")
	}
	ipt.newChainCalled = true
	if ipt.newChainErr != nil {
		return ipt.newChainErr
	}
	ipt.table = table
	ipt.chain = chain
	return nil
}

func (ipt *mockIPTablesChain) Exists(table string, chain string, rulespec ...string) (bool, error) {
	return ipt.table == table &&
		ipt.chain == chain &&
		slices.Equal(rulespec, ipt.rules), ipt.existsErr
}

func (ipt *mockIPTablesChain) Append(table string, chain string, rulespec ...string) error {
	if ipt.appendCalled {
		return errors.New("ipt.Append should only be called once")
	}
	ipt.appendCalled = true
	if ipt.table != table || ipt.chain != chain {
		return errors.New("should only be Append-ing to the one chain")
	}
	ipt.rules = rulespec
	return ipt.appendErr
}
