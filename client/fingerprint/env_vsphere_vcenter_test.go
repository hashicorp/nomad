// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
)

/*
These tests use the govmomi in-process VPX simulator — no real vCenter, no network, runs on all platforms.

The simulator creates a single vCenter with this inventory:

	DC0                     ← Datacenter
	  DC0_C0                ← ClusterComputeResource
	    DC0_C0_H{0,1,2}     ← HostSystem (3 cluster hosts)
	    DC0_C0_RP0_VM0      ← VirtualMachine  (vm.Config.Uuid = BIOS UUID)
	    DC0_C0_RP0_VM1      ← VirtualMachine
	    Resources           ← root ResourcePool
	  DC0_H0                ← standalone HostSystem
	LocalDS_0               ← Datastore
*/

// newTestVSphereClient constructs a vSphereClient directly from a govmomi
// client returned by the simulator. This bypasses newVSphereClient (which
// requires a real URL) and is only used in tests.
func newTestVSphereClient(c *govmomi.Client) *vSphereClient {
	return &vSphereClient{
		client: c,
		pc:     property.DefaultCollector(c.Client),
		logger: testlog.HCLogger(nil),
	}
}

// simulatorVMWithHostParent scans the VPX simulator inventory and returns the
// BIOS UUID and display name of the first VM whose ESXi host has a parent
// object of the given type (e.g. "ClusterComputeResource" or "ComputeResource").
//
// vm.Config.Uuid is the BIOS UUID — the same value the fingerprinter reads from
// /sys/class/dmi/id/product_uuid on a real VMware guest.
func simulatorVMWithHostParent(t *testing.T, ctx context.Context, c *govmomi.Client, hostParentType string) (biosUUID, vmName string) {
	t.Helper()

	finder := find.NewFinder(c.Client, false)
	dc, err := finder.DefaultDatacenter(ctx)
	must.NoError(t, err)
	finder.SetDatacenter(dc)

	vms, err := finder.VirtualMachineList(ctx, "*")
	must.NoError(t, err)
	must.Positive(t, len(vms))

	pc := property.DefaultCollector(c.Client)

	for _, vm := range vms {
		var v mo.VirtualMachine
		must.NoError(t, pc.RetrieveOne(ctx, vm.Reference(), []string{"config", "runtime"}, &v))
		must.NotNil(t, v.Config)

		if v.Runtime.Host == nil {
			continue
		}
		var h mo.HostSystem
		must.NoError(t, pc.RetrieveOne(ctx, *v.Runtime.Host, []string{"parent"}, &h))
		if h.Parent != nil && h.Parent.Type == hostParentType {
			return v.Config.Uuid, v.Config.Name
		}
	}

	t.Fatalf("simulatorVMWithHostParent: no VM with host parent type %q found in simulator inventory", hostParentType)
	return "", ""
}

// TestVSphereClient_fetchInventory_full verifies that fetchInventory correctly
// resolves all seven inventory fields against the in-process VPX simulator.
func TestVSphereClient_fetchInventory_full(t *testing.T) {
	ci.Parallel(t)

	model := simulator.VPX()
	must.NoError(t, model.Create())
	defer model.Remove()

	s := model.Service.NewServer()
	defer s.Close()

	ctx := context.Background()
	govmomiClient, err := govmomi.NewClient(ctx, s.URL, true)
	must.NoError(t, err)
	defer govmomiClient.Logout(ctx) //nolint:errcheck

	biosUUID, wantVMName := simulatorVMWithHostParent(t, ctx, govmomiClient, "ClusterComputeResource")

	c := newTestVSphereClient(govmomiClient)
	inv, err := c.fetchInventory(ctx, biosUUID)

	must.NoError(t, err)
	must.NotNil(t, inv)

	// VM identity.
	must.Eq(t, wantVMName, inv.VMName)
	must.NotEq(t, "", inv.VCenterUUID)

	// Inventory placement — all resolved from the VPX default topology.
	must.Eq(t, "DC0", inv.Datacenter)
	must.Eq(t, "DC0_C0", inv.Cluster)
	must.Eq(t, "Resources", inv.ResourcePool)
	must.NotEq(t, "", inv.Host)
	must.Eq(t, "LocalDS_0", inv.Datastore)
}

// TestVSphereClient_fetchInventory_vmNotFound verifies that fetchInventory
// returns a hard error when the UUID does not match any VM in vCenter.
func TestVSphereClient_fetchInventory_vmNotFound(t *testing.T) {
	ci.Parallel(t)

	model := simulator.VPX()
	must.NoError(t, model.Create())
	defer model.Remove()

	s := model.Service.NewServer()
	defer s.Close()

	ctx := context.Background()
	govmomiClient, err := govmomi.NewClient(ctx, s.URL, true)
	must.NoError(t, err)
	defer govmomiClient.Logout(ctx) //nolint:errcheck

	c := newTestVSphereClient(govmomiClient)
	inv, err := c.fetchInventory(ctx, "00000000-0000-0000-0000-000000000000")

	must.Error(t, err)
	must.Nil(t, inv)
	must.StrContains(t, err.Error(), "not found in vCenter")
}

// TestVSphereClient_fetchInventory_standaloneHost verifies that fetchInventory
// correctly handles a VM on a standalone ESXi host that is not part of any
// cluster. The Host field must be populated and the Cluster field must be
// empty — the cluster attribute is simply not published in this topology.
func TestVSphereClient_fetchInventory_standaloneHost(t *testing.T) {
	ci.Parallel(t)

	model := simulator.VPX()
	must.NoError(t, model.Create())
	defer model.Remove()

	s := model.Service.NewServer()
	defer s.Close()

	ctx := context.Background()
	govmomiClient, err := govmomi.NewClient(ctx, s.URL, true)
	must.NoError(t, err)
	defer govmomiClient.Logout(ctx) //nolint:errcheck

	biosUUID, wantVMName := simulatorVMWithHostParent(t, ctx, govmomiClient, "ComputeResource")

	c := newTestVSphereClient(govmomiClient)
	inv, err := c.fetchInventory(ctx, biosUUID)

	must.NoError(t, err)
	must.NotNil(t, inv)

	// VM identity.
	must.Eq(t, wantVMName, inv.VMName)

	// Host is populated; Cluster is empty because the host has no cluster parent.
	must.NotEq(t, "", inv.Host)
	must.Eq(t, "", inv.Cluster)

	// Other inventory fields are still resolved.
	must.Eq(t, "DC0", inv.Datacenter)
	must.Eq(t, "LocalDS_0", inv.Datastore)
}
