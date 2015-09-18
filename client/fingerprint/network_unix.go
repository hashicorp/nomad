// +build linux darwin
package fingerprint

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UnixNetworkFingerprint is used to fingerprint the Network capabilities of a node
type UnixNetworkFingerprint struct {
	logger *log.Logger
}

// NewNetworkFingerprint is used to create a CPU fingerprint
func NewNetworkFingerprinter(logger *log.Logger) NetworkFingerPrinter {
	f := &UnixNetworkFingerprint{logger: logger}
	return f
}

func (f *UnixNetworkFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	if ip := ifConfig("eth0"); ip != "" {
		node.Attributes["network.ip-address"] = ip
	}

	if s := f.LinkSpeed("eth0"); s != "" {
		node.Attributes["network.throughput"] = s
	}

	// return true, because we have a network connection
	return true, nil
}

func (f *UnixNetworkFingerprint) Interfaces() []string {
	// No OP for now
	return nil
}

// LinkSpeed attempts to determine link speed, first by checking if any tools
// exist that can return the speed (ethtool for now). If no tools are found,
// fall back to /sys/class/net speed file, if it exists.
//
// The return value is in the format of "<int>MB/s"
//
// LinkSpeed returns an empty string if no tools or sys file are found
func (f *UnixNetworkFingerprint) LinkSpeed(device string) string {
	// Use LookPath to find the ethtool in the systems $PATH
	// If it's not found or otherwise errors, LookPath returns and empty string
	// and an error we can ignore for our purposes
	ethtoolPath, _ := exec.LookPath("ethtool")
	if ethtoolPath != "" {
		speed := linkSpeedEthtool(ethtoolPath, device)
		if speed != "" {
			return speed
		}
	}
	fmt.Println("[WARN] Ethtool not found, checking /sys/net speed file")

	// Fall back on checking a system file for link speed.
	return linkSpeedSys(device)
}

// linkSpeedSys parses the information stored in the sys diretory for the
// default device. This method retuns an empty string if the file is not found
// or cannot be read
func linkSpeedSys(device string) string {
	path := fmt.Sprintf("/sys/class/net/%s/speed", device)
	_, err := os.Stat(path)
	if err != nil {
		log.Printf("[WARN] Error getting information about net speed")
		return ""
	}

	// Read contents of the device/speed file
	content, err := ioutil.ReadFile(path)
	if err == nil {
		lines := strings.Split(string(content), "\n")
		// convert to MB/s
		mbs, err := strconv.Atoi(lines[0])
		if err != nil {
			log.Println("[WARN] Unable to parse ethtool output")
			return ""
		}
		mbs = mbs / 8

		return fmt.Sprintf("%dMB/s", mbs)
	}
	return ""
}

// linkSpeedEthtool uses the ethtool installed on the node to gather link speed
// information. It executes the command on the device specified and parses
// out the speed. The expected format is Mbps and converted to MB/s
// Returns an empty string there is an error in parsing or executing ethtool
func linkSpeedEthtool(path, device string) string {
	outBytes, err := exec.Command(path, device).Output()
	if err == nil {
		output := strings.TrimSpace(string(outBytes))
		re := regexp.MustCompile("Speed: [0-9]+[a-zA-Z]+/s")
		m := re.FindString(output)
		if m == "" {
			// no matches found, output may be in a different format
			log.Println("[WARN] Ethtool output did not match regex")
			return ""
		}

		// Split and trim the Mb/s unit from the string output
		args := strings.Split(m, ": ")
		raw := strings.TrimSuffix(args[1], "Mb/s")

		// convert to MB/s
		mbs, err := strconv.Atoi(raw)
		if err != nil {
			log.Println("[WARN] Unable to parse ethtool output")
			return ""
		}
		mbs = mbs / 8

		return fmt.Sprintf("%dMB/s", mbs)
	}
	log.Printf("error calling ethtool (%s): %s", path, err)
	return ""
}

// ifConfig returns the IP Address for this node according to ifConfig, for the
// specified device.
func ifConfig(device string) string {
	ifConfigPath, _ := exec.LookPath("ifconfig")
	if ifConfigPath != "" {
		outBytes, err := exec.Command(ifConfigPath, device).Output()
		if err == nil {
			output := strings.TrimSpace(string(outBytes))
			re := regexp.MustCompile("inet addr:[0-9].+")
			m := re.FindString(output)
			args := strings.Split(m, "inet addr:")

			return args[1]
		}
		log.Printf("[Err] Error calling ifconfig (%s): %s", ifConfigPath, err)
		return ""
	}

	log.Println("[WARN] Ethtool not found")
	return ""
}
