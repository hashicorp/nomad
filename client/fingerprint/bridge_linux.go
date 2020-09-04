package fingerprint

import (
	"bufio"
	"fmt"
	"os"
	"regexp"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/host"
)

const bridgeKernelModuleName = "bridge"

func (f *BridgeFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	if err := f.checkKMod(bridgeKernelModuleName); err != nil {
		f.logger.Warn("failed to detect bridge kernel module, bridge network mode disabled", "error", err)
		return nil
	}

	resp.NodeResources = &structs.NodeResources{
		Networks: []*structs.NetworkResource{
			{
				Mode: "bridge",
			},
		},
		NodeNetworks: []*structs.NodeNetworkResource{
			{
				Mode:   "bridge",
				Device: req.Config.BridgeNetworkName,
			},
		},
	}
	resp.Detected = true
	return nil
}

func (f *BridgeFingerprint) checkKMod(mod string) error {
	hostInfo, err := host.Info()
	if err != nil {
		return err
	}

	dynErr := f.checkKModFile(mod, "/proc/modules", fmt.Sprintf("%s\\s+.*$", mod))
	if dynErr == nil {
		return nil
	}

	builtinErr := f.checkKModFile(mod,
		fmt.Sprintf("/lib/modules/%s/modules.builtin", hostInfo.KernelVersion),
		fmt.Sprintf(".+\\/%s.ko$", mod))
	if builtinErr == nil {
		return nil
	}

	return fmt.Errorf("%v, %v", dynErr, builtinErr)
}

func (f *BridgeFingerprint) checkKModFile(mod, fileName, pattern string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("could not read %s: %v", fileName, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if matched, err := regexp.MatchString(pattern, scanner.Text()); matched {
			return nil
		} else if err != nil {
			return fmt.Errorf("could not parse %s: %v", fileName, err)
		}
	}

	return fmt.Errorf("could not detect kernel module %s", mod)
}
