package fingerprint

import (
	"log"
	"runtime"
	"strconv"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
)

// OSFingerprint is used to fingerprint the operating system
type OSFingerprint struct {
	logger *log.Logger
}

// NewOSFingerprint is used to create an OS fingerprint
func NewOSFingerprint(logger *log.Logger) Fingerprint {
	f := &OSFingerprint{logger}
	return f
}

func (f *OSFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	node.Attributes["os"] = runtime.GOOS
	f.logger.Printf("[DEBUG] fingerprint.os: detected '%s'", runtime.GOOS)

	cpuInfo, err := cpu.CPUInfo()
	if err != nil {
		f.logger.Println("[WARN] Error reading CPU information:", err)
	}

	var numCores int32
	numCores = 0
	var modelName string
	// Assume all CPUs found have same Model. Log if not.
	// If CPUInfo() returns err above, this loop is still safe
	for i, c := range cpuInfo {
		f.logger.Printf("(%d) Vendor: %s", i, c.VendorID)
		numCores += c.Cores
		if modelName != "" && modelName != c.ModelName {
			f.logger.Println("[WARN] Found different model names in the same CPU information. Recording last found")
		}
		modelName = c.ModelName
	}
	if numCores > 0 {
		node.Attributes["cpu.numcores"] = strconv.FormatInt(int64(numCores), 10)
	}
	if modelName != "" {
		node.Attributes["cpu.modelname"] = modelName
	}

	hostInfo, err := host.HostInfo()
	if err != nil {
		f.logger.Println("[WARN] Error retrieving host information: ", err)
	} else {
		node.Attributes["os.name"] = hostInfo.Platform
		node.Attributes["os.version"] = hostInfo.PlatformVersion
		node.Attributes["hostname"] = hostInfo.Hostname
		node.Attributes["kernel.name"] = hostInfo.OS
	}

	f.logger.Printf("Node: %s", node)

	return true, nil
}
