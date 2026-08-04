// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"context"
	"fmt"
	"net/url"

	log "github.com/hashicorp/go-hclog"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// vSphereClient wraps a live govmomi session and exposes inventory queries.
type vSphereClient struct {
	client *govmomi.Client
	pc     *property.Collector
	logger log.Logger
}

// newVSphereClient connects to vCenter and returns a ready-to-use client.
func newVSphereClient(ctx context.Context, vcURL, user, password string, insecure bool, logger log.Logger) (*vSphereClient, error) {
	parsedURL, err := url.Parse(vcURL)
	if err != nil {
		return nil, fmt.Errorf("parsing vsphere.url %q: %w", vcURL, err)
	}
	parsedURL.User = url.UserPassword(user, password)

	client, err := govmomi.NewClient(ctx, parsedURL, insecure)
	if err != nil {
		return nil, fmt.Errorf("connecting to vCenter %q: %w", vcURL, err)
	}

	return &vSphereClient{
		client: client,
		pc:     property.DefaultCollector(client.Client),
		logger: logger,
	}, nil
}

// vmInventory holds all inventory metadata resolved from a single vCenter query.
// Empty string means the attribute was unavailable or could not be resolved.
type vmInventory struct {
	VMName       string
	VCenterUUID  string
	Datacenter   string
	Cluster      string
	ResourcePool string
	Host         string
	Datastore    string
}

// fetchInventory locates the VM by its BIOS UUID and resolves all inventory
// attributes into a vmInventory.
//
// Hard errors (VM not found, bulk property fetch failed) are returned to the
// caller. Supplementary attribute failures (MoRef resolution, datacenter walk)
// are logged at Debug and skipped.
func (c *vSphereClient) fetchInventory(ctx context.Context, vmUUID string) (*vmInventory, error) {
	// FindByUuid with nil datacenter searches all datacenters; vmSearch=true
	// and instanceUuid=nil select the BIOS UUID (not the instance UUID).
	si := object.NewSearchIndex(c.client.Client)
	ref, err := si.FindByUuid(ctx, nil, vmUUID, true, nil)
	if err != nil {
		return nil, fmt.Errorf("finding VM by UUID %q: %w", vmUUID, err)
	}
	if ref == nil {
		return nil, fmt.Errorf("VM with UUID %q not found in vCenter", vmUUID)
	}
	vmRef := ref.Reference()

	// Fetch all required VM properties in a single SOAP round-trip.
	var vm mo.VirtualMachine
	if err := c.pc.RetrieveOne(ctx, vmRef, []string{"config.name", "resourcePool", "runtime.host", "datastore"}, &vm); err != nil {
		return nil, fmt.Errorf("retrieving properties for VM %q: %w", vmUUID, err)
	}

	inv := &vmInventory{
		VCenterUUID: c.client.ServiceContent.About.InstanceUuid,
	}
	if vm.Config != nil {
		inv.VMName = vm.Config.Name
	}

	inv.ResourcePool = c.resolveResourcePool(ctx, vm.ResourcePool)
	inv.Host, inv.Cluster = c.resolveHostAndCluster(ctx, vm.Runtime.Host)
	inv.Datastore = c.resolveDatastore(ctx, vm.Datastore)
	inv.Datacenter = c.resolveDatacenter(ctx, vmRef)

	return inv, nil
}

// resolveResourcePool returns the name of the given resource pool MoRef.
func (c *vSphereClient) resolveResourcePool(ctx context.Context, ref *types.ManagedObjectReference) string {
	if ref == nil {
		return ""
	}
	var rp mo.ResourcePool
	if err := c.pc.RetrieveOne(ctx, *ref, []string{"name"}, &rp); err != nil {
		c.logger.Debug("could not resolve resource pool name, skipping attribute", "error", err)
		return ""
	}
	return rp.Name
}

// resolveHostAndCluster returns the ESXi host name and, when the host belongs
// to a DRS/HA cluster, the cluster name. Standalone hosts return ("name", "").
func (c *vSphereClient) resolveHostAndCluster(ctx context.Context, ref *types.ManagedObjectReference) (host, cluster string) {
	if ref == nil {
		return "", ""
	}
	var h mo.HostSystem
	if err := c.pc.RetrieveOne(ctx, *ref, []string{"name", "parent"}, &h); err != nil {
		c.logger.Debug("could not resolve ESXi host name, skipping attribute", "error", err)
		return "", ""
	}
	host = h.Name

	// host.Parent is ClusterComputeResource for cluster hosts; plain
	// ComputeResource for standalone hosts — skip cluster in the latter case.
	if h.Parent != nil && h.Parent.Type == "ClusterComputeResource" {
		var cl mo.ClusterComputeResource
		if err := c.pc.RetrieveOne(ctx, *h.Parent, []string{"name"}, &cl); err != nil {
			c.logger.Debug("could not resolve cluster name, skipping attribute", "error", err)
		} else {
			cluster = cl.Name
		}
	}
	return host, cluster
}

// resolveDatastore returns the name of the first datastore in the list.
// VMs commonly have one primary datastore even when additional disks span
// multiple datastores.
func (c *vSphereClient) resolveDatastore(ctx context.Context, refs []types.ManagedObjectReference) string {
	if len(refs) == 0 {
		return ""
	}
	var ds mo.Datastore
	if err := c.pc.RetrieveOne(ctx, refs[0], []string{"name"}, &ds); err != nil {
		c.logger.Debug("could not resolve datastore name, skipping attribute", "error", err)
		return ""
	}
	return ds.Name
}

// resolveDatacenter walks the inventory ancestor chain of vmRef and returns
// the name of the enclosing Datacenter object. mo.Ancestors fetches the full
// parent chain in one SOAP call; scanning for type "Datacenter" is the only
// approach that works correctly across nested folder structures.
func (c *vSphereClient) resolveDatacenter(ctx context.Context, vmRef types.ManagedObjectReference) string {
	ancestors, err := mo.Ancestors(ctx, c.client.Client, c.client.ServiceContent.PropertyCollector, vmRef)
	if err != nil {
		c.logger.Debug("could not walk VM ancestor chain for datacenter, skipping attribute", "error", err)
		return ""
	}
	for _, ancestor := range ancestors {
		if ancestor.Self.Type == "Datacenter" {
			return ancestor.Name
		}
	}
	return ""
}
