package fingerprint

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// StorageFingerprint is used to measure the amount of storage free for
// applications that the Nomad agent will run on this machine.
type StorageFingerprint struct {
	logger *log.Logger
}

var (
	reWindowsTotalSpace = regexp.MustCompile("Total # of bytes\\s+: (\\d+)")
	reWindowsFreeSpace  = regexp.MustCompile("Total # of free bytes\\s+: (\\d+)")
)

func NewStorageFingerprint(logger *log.Logger) Fingerprint {
	fp := &StorageFingerprint{logger: logger}
	return fp
}

func (f *StorageFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {

	// Initialize these to empty defaults
	node.Attributes["storage.volume"] = ""
	node.Attributes["storage.bytestotal"] = ""
	node.Attributes["storage.bytesfree"] = ""
	if node.Resources == nil {
		node.Resources = &structs.Resources{}
	}

	if runtime.GOOS == "windows" {
		path, err := filepath.Abs(cfg.AllocDir)
		if err != nil {
			return false, fmt.Errorf("Failed to detect volume for storage directory %s: %s", cfg.AllocDir, err)
		}
		volume := filepath.VolumeName(path)
		node.Attributes["storage.volume"] = volume
		out, err := exec.Command("fsutil", "volume", "diskfree", volume).Output()
		if err != nil {
			return false, fmt.Errorf("Failed to inspect free space from volume %s: %s", volume, err)
		}
		outstring := string(out)

		totalMatches := reWindowsTotalSpace.FindStringSubmatch(outstring)
		if len(totalMatches) == 2 {
			node.Attributes["storage.bytestotal"] = totalMatches[1]
			total, err := strconv.ParseInt(totalMatches[1], 10, 64)
			if err != nil {
				return false, fmt.Errorf("Failed to parse storage.bytestotal in bytes: %s", err)
			}
			// Convert from bytes to to MB
			node.Resources.DiskMB = int(total / 1024 / 1024)
		} else {
			return false, fmt.Errorf("Failed to parse output from fsutil")
		}

		freeMatches := reWindowsFreeSpace.FindStringSubmatch(outstring)
		if len(freeMatches) == 2 {
			node.Attributes["storage.bytesfree"] = freeMatches[1]
			_, err := strconv.ParseInt(freeMatches[1], 10, 64)
			if err != nil {
				return false, fmt.Errorf("Failed to parse storage.bytesfree in bytes: %s", err)
			}

		} else {
			return false, fmt.Errorf("Failed to parse output from fsutil")
		}
	} else {
		path, err := filepath.Abs(cfg.AllocDir)
		if err != nil {
			return false, fmt.Errorf("Failed to determine absolute path for %s", cfg.AllocDir)
		}

		// Use -k to standardize the output values between darwin and linux
		mountOutput, err := exec.Command("df", "-k", path).Output()
		if err != nil {
			return false, fmt.Errorf("Failed to determine mount point for %s", path)
		}
		// Output looks something like:
		//	Filesystem 1024-blocks      Used Available Capacity   iused    ifree %iused  Mounted on
		//	/dev/disk1   487385240 423722532  63406708    87% 105994631 15851677   87%   /
		//	[0] volume [1] capacity [2] SKIP  [3] free
		fields := strings.Fields(strings.Split(string(mountOutput), "\n")[1])
		node.Attributes["storage.volume"] = fields[0]

		total, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return false, fmt.Errorf("Failed to parse storage.bytestotal size in kilobytes")
		}
		node.Attributes["storage.bytestotal"] = strconv.FormatInt(total*1024, 10)

		free, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return false, fmt.Errorf("Failed to parse storage.bytesfree size in kilobytes")
		}
		// Convert from KB to MB
		node.Resources.DiskMB = int(free / 1024)
		// Convert from KB to bytes
		node.Attributes["storage.bytesfree"] = strconv.FormatInt(free*1024, 10)
	}

	return true, nil
}
