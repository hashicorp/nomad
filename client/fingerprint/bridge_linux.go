package fingerprint

import (
	"bufio"
	"fmt"
	"os"
	"regexp"

	"github.com/hashicorp/nomad/nomad/structs"
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
	file, err := os.Open("/proc/modules")
	if err != nil {
		return fmt.Errorf("could not read /proc/modules: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	pattern := fmt.Sprintf("%s\\s+.*$", mod)
	for scanner.Scan() {
		if matched, err := regexp.MatchString(pattern, scanner.Text()); matched {
			return nil
		} else if err != nil {
			return fmt.Errorf("could not parse /proc/modules: %v", err)
		}
	}

	return fmt.Errorf("could not detect kernel module %s", mod)
}
