package fingerprint

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), mod+" ") {
			return nil
		}
	}

	return fmt.Errorf("could not detect kernel module %s", mod)
}
