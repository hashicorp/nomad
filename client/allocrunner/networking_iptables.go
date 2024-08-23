// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package allocrunner

import (
	"fmt"

	"github.com/coreos/go-iptables/iptables"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// cniAdminChainName is the name of the admin iptables chain used to allow
	// forwarding traffic to allocations
	cniAdminChainName = "NOMAD-ADMIN"
)

// newIPTables provides an *iptables.IPTables for the requested address family
// "ipv6" or "ipv4"
func newIPTables(family structs.NodeNetworkAF) (IPTables, error) {
	if family == structs.NodeNetworkAF_IPv6 {
		return iptables.New(iptables.IPFamily(iptables.ProtocolIPv6), iptables.Timeout(5))
	}
	return iptables.New()
}
func newIPTablesCleanup(family structs.NodeNetworkAF) (IPTablesCleanup, error) {
	return newIPTables(family)
}
func newIPTablesChain(family structs.NodeNetworkAF) (IPTablesChain, error) {
	return newIPTables(family)
}

// IPTables is a subset of iptables.IPTables
type IPTables interface {
	IPTablesCleanup
	IPTablesChain
}
type IPTablesCleanup interface {
	List(table, chain string) ([]string, error)
	Delete(table, chain string, rule ...string) error
	ClearAndDeleteChain(table, chain string) error
}
type IPTablesChain interface {
	ListChains(table string) ([]string, error)
	NewChain(table string, chain string) error
	Exists(table string, chain string, rulespec ...string) (bool, error)
	Append(table string, chain string, rulespec ...string) error
}

// ensureChainRule ensures our admin chain exists and contains a rule to accept
// traffic to the bridge network
func ensureChainRule(ipt IPTablesChain, bridgeName, subnet string) error {
	if err := ensureChain(ipt, "filter", cniAdminChainName); err != nil {
		return err
	}
	rule := generateAdminChainRule(bridgeName, subnet)
	if err := appendChainRule(ipt, cniAdminChainName, rule); err != nil {
		return err
	}
	return nil
}

// ensureChain ensures that the given chain exists, creating it if missing
func ensureChain(ipt IPTablesChain, table, chain string) error {
	chains, err := ipt.ListChains(table)
	if err != nil {
		return fmt.Errorf("failed to list iptables chains: %v", err)
	}
	for _, ch := range chains {
		if ch == chain {
			return nil
		}
	}

	err = ipt.NewChain(table, chain)

	// if err is for chain already existing return as it is possible another
	// goroutine created it first
	if e, ok := err.(*iptables.Error); ok && e.ExitStatus() == 1 {
		return nil
	}

	return err
}

// appendChainRule adds the given rule to the chain
func appendChainRule(ipt IPTablesChain, chain string, rule []string) error {
	exists, err := ipt.Exists("filter", chain, rule...)
	if !exists && err == nil {
		err = ipt.Append("filter", chain, rule...)
	}
	return err
}

// generateAdminChainRule builds the iptables rule that is inserted into the
// CNI admin chain to ensure traffic forwarding to the bridge network
func generateAdminChainRule(bridgeName, subnet string) []string {
	return []string{"-o", bridgeName, "-d", subnet, "-j", "ACCEPT"}
}
