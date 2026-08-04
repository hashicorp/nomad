// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"os"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// vsphereFingerprinterName is the name of the vSphere fingerprinter and is
	// used in configuration and logging.
	vsphereFingerprinterName = "env_vsphere"

	// vsphereDMIVendor is the sys_vendor string written by the VMware hypervisor
	// into the VM's SMBIOS tables. It is the probe value that confirms we are
	// running inside a VMware guest.
	vsphereDMIVendor = "VMware, Inc."
)

// dmiBase is the sysfs directory where the Linux kernel exposes DMI/SMBIOS
// fields. It is declared here (without a build tag) so tests on any platform
// can override it to point at fixture files. The Linux readDMI implementation
// reads from this path; the non-Linux stub ignores it entirely.
var dmiBase = "/sys/class/dmi/id"

/*
             vSphere Fingerprinter — Architecture Overview

Unlike AWS, Azure, and GCE, VMware vSphere has no link-local HTTP metadata
endpoint inside the guest OS. This fingerprinter uses a two-tier approach to
work around that constraint.

Tier 1 (always active): reads DMI/SMBIOS files from the Linux kernel sysfs to
detect the VMware hypervisor and collect the VM's UUID. Requires no network,
no credentials, and no VMware Tools.

Tier 2 (opt-in): queries the vSphere API via govmomi to enrich the node with
inventory metadata — datacenter, cluster, resource pool, and more. Activates
only when vsphere.url is set in the client options block. Tier 2 failure is
non-fatal; Tier 1 attributes are always preserved.

The VM UUID produced by Tier 1 is the join key that the nomad-autoscaler
vSphere target plugin uses to map a Nomad node back to its vSphere VM object.
*/

// EnvVSphereFingerprint is used to fingerprint VMware vSphere VMs.
type EnvVSphereFingerprint struct {
	StaticFingerprinter
	logger log.Logger
}

// NewEnvVSphereFingerprint is used to create a fingerprint from vSphere
// metadata. It wraps the fingerprinter in a retry wrapper.
func NewEnvVSphereFingerprint(logger log.Logger) Fingerprint {
	namedLogger := logger.Named(vsphereFingerprinterName)
	return NewRetryWrapper(
		&EnvVSphereFingerprint{
			logger: namedLogger,
		},
		namedLogger,
		vsphereFingerprinterName,
	)
}

// Fingerprint implements the Fingerprint interface. It runs Tier 1 on every invocation
// and, when vsphere.url is configured, proceeds to Tier 2.
func (f *EnvVSphereFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	// Confirm we are inside a VMware guest by checking the hypervisor-written sys_vendor field.
	vendor, err := readDMI("sys_vendor")
	if err != nil {
		if os.IsPermission(err) {
			f.logger.Warn("permission denied reading DMI sys_vendor — ensure the Nomad agent runs as root",
				"path", "sys_vendor", "error", err)
		}
		return wrapProbeError(fmt.Errorf("reading DMI sys_vendor: %w", err))
	}

	if vendor != vsphereDMIVendor {
		return wrapProbeError(fmt.Errorf("not a VMware guest: sys_vendor=%q", vendor))
	}

	// product_uuid is the VM's UUID and the join key for Tier 2 govmomi lookups.
	// On Linux this file is root-readable only (mode 0400), hence the explicit
	// permission check and Warn below.
	vmUUID, err := readDMI("product_uuid")
	if err != nil {
		if os.IsPermission(err) {
			f.logger.Warn("permission denied reading DMI product_uuid — ensure the Nomad agent runs as root",
				"path", "product_uuid", "error", err)
		}
		return wrapProbeError(fmt.Errorf("reading DMI product_uuid: %w", err))
	}
	if vmUUID == "" {
		return wrapProbeError(fmt.Errorf("DMI product_uuid is empty"))
	}

	productName, err := readDMI("product_name")
	if err != nil {
		f.logger.Debug("could not read DMI product_name, skipping attribute", "error", err)
	}

	biosVersion, err := readDMI("bios_version")
	if err != nil {
		f.logger.Debug("could not read DMI bios_version, skipping attribute", "error", err)
	}

	resp.AddAttribute(structs.UniqueNamespace("platform.vsphere.vm-uuid"), vmUUID)
	resp.AddAttribute("platform.vsphere.sys-vendor", vendor)
	if productName != "" {
		resp.AddAttribute("platform.vsphere.product-name", productName)
	}
	if biosVersion != "" {
		resp.AddAttribute("platform.vsphere.bios-version", biosVersion)
	}
	resp.Detected = true

	// Todo: implement Tier 2.

	return nil
}

// Reload is a no-op but satisfies the ReloadableFingerprint interface.
func (f *EnvVSphereFingerprint) Reload() {}
