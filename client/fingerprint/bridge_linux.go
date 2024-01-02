// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package fingerprint

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/v3/host"
)

const bridgeKernelModuleName = "bridge"

const (
	dynamicModuleRe = `%s\s+.*$`
	builtinModuleRe = `.+/%s.ko$`
	dependsModuleRe = `.+/%s.ko(\.xz)?:.*$`
)

func (f *BridgeFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	if err := f.detect(bridgeKernelModuleName); err != nil {
		f.logger.Warn("failed to detect bridge kernel module, bridge network mode disabled", "error", err)
		return nil
	}

	resp.NodeResources = &structs.NodeResources{
		Networks: []*structs.NetworkResource{{
			Mode: "bridge",
		}},
		NodeNetworks: []*structs.NodeNetworkResource{{
			Mode:   "bridge",
			Device: req.Config.BridgeNetworkName,
		}},
	}

	resp.AddAttribute("nomad.bridge.hairpin_mode",
		strconv.FormatBool(req.Config.BridgeNetworkHairpinMode))

	resp.Detected = true
	return nil
}

func (f *BridgeFingerprint) regexp(pattern, module string) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(pattern, module))
}

func (f *BridgeFingerprint) detect(module string) error {
	// accumulate errors from every place we might find the module
	var errs error

	// Check if the module is in /sys/modules
	sysfsModulePath := fmt.Sprintf("/sys/module/%s", module)
	if err := f.findDir(sysfsModulePath); err != nil {
		errs = multierror.Append(errs, err)
	} else {
		return nil
	}

	// check if the module has been dynamically loaded
	dynamicPath := "/proc/modules"
	if err := f.searchFile(module, dynamicPath, f.regexp(dynamicModuleRe, module)); err != nil {
		errs = multierror.Append(errs, err)
	} else {
		return nil
	}

	// will need kernel info to look for builtin and unloaded modules
	hostInfo, err := host.Info()
	if err != nil {
		return err
	}

	// check if the module is builtin to the kernel
	builtinPath := fmt.Sprintf("/lib/modules/%s/modules.builtin", hostInfo.KernelVersion)
	if err := f.searchFile(module, builtinPath, f.regexp(builtinModuleRe, module)); err != nil {
		errs = multierror.Append(errs, err)
	} else {
		return nil
	}

	// check if the module is dynamic but unloaded (will have a dep entry)
	dependsPath := fmt.Sprintf("/lib/modules/%s/modules.dep", hostInfo.KernelVersion)
	if err := f.searchFile(module, dependsPath, f.regexp(dependsModuleRe, module)); err != nil {
		errs = multierror.Append(errs, err)
	} else {
		return nil
	}

	return errs
}

func (f *BridgeFingerprint) findDir(dirname string) error {
	if _, err := os.Stat(dirname); err != nil {
		return fmt.Errorf("failed to find %s: %v", dirname, err)
	} else {
		return nil
	}
}

func (f *BridgeFingerprint) searchFile(module, filename string, re *regexp.Regexp) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", filename, err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if re.MatchString(scanner.Text()) {
			return nil // found the module!
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan %s: %v", filename, err)
	}

	return fmt.Errorf("module %s not in %s", module, filename)
}
