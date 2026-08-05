// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// dmiFixturePath returns the absolute path to a test fixture directory for
// DMI files. The fingerprinter's package-level dmiBase variable is overridden
// in each test to point at these fixture directories instead of the real
// /sys/class/dmi/id path.
func dmiFixturePath(t *testing.T, variant string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "test_fixtures", "dmi", variant)
}

func Test_NewEnvVSphereFingerprint(t *testing.T) {
	ci.Parallel(t)

	f := NewEnvVSphereFingerprint(testlog.HCLogger(t))
	must.NotNil(t, f)

	retryWrapper, ok := f.(*RetryWrapper)
	must.True(t, ok)
	must.Eq(t, vsphereFingerprinterName, retryWrapper.name)

	_, ok = retryWrapper.fingerprinter.(*EnvVSphereFingerprint)
	must.True(t, ok)
}

// TestEnvVSphereFingerprint_nonVMware verifies that the fingerprinter silently
// skips when the DMI sys_vendor value is not "VMware, Inc." (e.g. on AWS).
func TestEnvVSphereFingerprint_nonVMware(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("DMI sysfs is Linux-only")
	}
	ci.Parallel(t)

	// Point dmiBase at the "other" fixture (sys_vendor = "Amazon EC2").
	orig := dmiBase
	dmiBase = dmiFixturePath(t, "other")
	t.Cleanup(func() { dmiBase = orig })

	f := NewEnvVSphereFingerprint(testlog.HCLogger(t))
	node := &structs.Node{Attributes: make(map[string]string)}
	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse

	err := f.Fingerprint(request, &response)
	must.NoError(t, err)
	must.False(t, response.Detected)
	must.MapEmpty(t, response.Attributes)
}

// TestEnvVSphereFingerprint_dmiPermissionDenied verifies that a permission
// error on sys_vendor results in a Warn log and a silent skip (not a hard
// error that would prevent agent startup).
func TestEnvVSphereFingerprint_dmiPermissionDenied(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("DMI sysfs is Linux-only")
	}
	if os.Getuid() == 0 {
		t.Skip("cannot simulate permission denied when running as root")
	}
	ci.Parallel(t)

	// Create a temp directory with a sys_vendor file that has no read permission.
	tmpDir := t.TempDir()
	vendorFile := filepath.Join(tmpDir, "sys_vendor")
	must.NoError(t, os.WriteFile(vendorFile, []byte("VMware, Inc.\n"), 0000))

	orig := dmiBase
	dmiBase = tmpDir
	t.Cleanup(func() { dmiBase = orig })

	f := NewEnvVSphereFingerprint(testlog.HCLogger(t))
	node := &structs.Node{Attributes: make(map[string]string)}
	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse

	// Must not return an error — agent must always start.
	err := f.Fingerprint(request, &response)
	must.NoError(t, err)
	must.False(t, response.Detected)
	must.MapEmpty(t, response.Attributes)
}

// TestEnvVSphereFingerprint_productUUIDPermissionDenied verifies that a
// permission error specifically on product_uuid (sys_vendor readable, UUID not)
// also results in a Warn log and a silent skip.
func TestEnvVSphereFingerprint_productUUIDPermissionDenied(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("DMI sysfs is Linux-only")
	}
	if os.Getuid() == 0 {
		t.Skip("cannot simulate permission denied when running as root")
	}
	ci.Parallel(t)

	tmpDir := t.TempDir()
	must.NoError(t, os.WriteFile(filepath.Join(tmpDir, "sys_vendor"), []byte("VMware, Inc.\n"), 0644))
	must.NoError(t, os.WriteFile(filepath.Join(tmpDir, "product_uuid"), []byte("4218a4dd-a9f3-8c2b-1e40-d3a5c2b9f7e1\n"), 0000))

	orig := dmiBase
	dmiBase = tmpDir
	t.Cleanup(func() { dmiBase = orig })

	f := NewEnvVSphereFingerprint(testlog.HCLogger(t))
	node := &structs.Node{Attributes: make(map[string]string)}
	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse

	err := f.Fingerprint(request, &response)
	must.NoError(t, err)
	must.False(t, response.Detected)
	must.MapEmpty(t, response.Attributes)
}

// TestEnvVSphereFingerprint_tier1Only verifies that a VMware VM with all DMI
// files readable publishes the correct Tier 1 attributes and sets Detected.
// No vCenter config is present so Tier 2 is not attempted.
func TestEnvVSphereFingerprint_tier1Only(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("DMI sysfs is Linux-only")
	}
	ci.Parallel(t)

	orig := dmiBase
	dmiBase = dmiFixturePath(t, "vmware")
	t.Cleanup(func() { dmiBase = orig })

	f := NewEnvVSphereFingerprint(testlog.HCLogger(t))
	node := &structs.Node{Attributes: make(map[string]string)}

	// No vsphere.url in options — Tier 2 must not be attempted.
	cfg := &config.Config{Options: map[string]string{}}
	request := &FingerprintRequest{Config: cfg, Node: node}
	var response FingerprintResponse

	err := f.Fingerprint(request, &response)
	must.NoError(t, err)
	must.True(t, response.Detected)

	assertNodeAttributeEquals(t, response.Attributes,
		"unique.platform.vsphere.vm-uuid", "4218a4dd-a9f3-8c2b-1e40-d3a5c2b9f7e1")
	assertNodeAttributeEquals(t, response.Attributes,
		"platform.vsphere.sys-vendor", "VMware, Inc.")
	assertNodeAttributeEquals(t, response.Attributes,
		"platform.vsphere.product-name", "VMware Virtual Platform")
	assertNodeAttributeEquals(t, response.Attributes,
		"platform.vsphere.bios-version", "6.00")

	// No Tier 2 attributes should be present.
	must.MapNotContainsKey(t, response.Attributes, "platform.vsphere.datacenter")
	must.MapNotContainsKey(t, response.Attributes, "platform.vsphere.cluster")
	must.MapNotContainsKey(t, response.Attributes, "platform.vsphere.resource-pool")
}
