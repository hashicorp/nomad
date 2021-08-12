package fingerprint

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

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
	if err := f.detect(req); err != nil {
		f.logger.Warn("failed to dtect bridge network setup, bridge network mode disabled", "error", err)
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

	resp.Detected = true
	return nil
}

func (f *BridgeFingerprint) regexp(pattern, module string) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(pattern, module))
}

func (f BridgeFingerprint) detect(req *FingerprintRequest) error {
	if err := f.detectKernelModule(bridgeKernelModuleName); err != nil {
		return fmt.Errorf("failed to detect bridge kernel module: %v", err)
	}

	return f.detectCNIBinaries(req.Config.CNIPath)
}

func (f *BridgeFingerprint) detectCNIBinaries(cniPath string) error {
	if cniPath == "" {
		return fmt.Errorf("cni is not configured")
	}

	var errs error

	// plugins required by in client/allocrunner/networking_bridge_linux.go.
	plugins := []string{"bridge", "firewall", "portmap", "host-local"}
	for _, plugin := range plugins {
		if err := f.checkCNIPluginBinary(cniPath, plugin); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

func (f *BridgeFingerprint) checkCNIPluginBinary(cniPath, plugin string) error {
	path := filepath.Join(cniPath, plugin)
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to find the %v CNI plugin in %v: %v", plugin, path, err)
	}

	if !fi.Mode().IsRegular() {
		return fmt.Errorf("the %v CNI plugin is not a regular file: %v", plugin, path)

	}

	return nil
}

func (f *BridgeFingerprint) detectKernelModule(module string) error {
	// accumulate errors from every place we might find the module
	var errs error

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
