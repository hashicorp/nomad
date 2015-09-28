// +build linux darwin

package fingerprint

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// NetworkFingerprint is used to fingerprint the Network capabilities of a node
type NetworkFingerprint struct {
	logger *log.Logger
}

// NewNetworkFingerprint returns a new NetworkFingerprinter with the given
// logger
func NewNetworkFingerprinter(logger *log.Logger) Fingerprint {
	f := &NetworkFingerprint{logger: logger}
	return f
}

func (f *NetworkFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// newNetwork is populated and addded to the Nodes resources
	newNetwork := &structs.NetworkResource{}

	// eth0 is the default device for Linux, and en0 is default for OS X
	defaultDevice := "eth0"
	if "darwin" == runtime.GOOS {
		defaultDevice = "en0"
	}

	newNetwork.Device = defaultDevice

	if ip := f.ifConfig(defaultDevice); ip != "" {
		node.Attributes["network.ip-address"] = ip
		newNetwork.IP = ip
		newNetwork.CIDR = newNetwork.IP + "/32"
	}

	if throughput := f.linkSpeed("eth0"); throughput > 0 {
		newNetwork.MBits = throughput
	}

	if node.Resources == nil {
		node.Resources = &structs.Resources{}
	}

	node.Resources.Networks = append(node.Resources.Networks, newNetwork)

	// return true, because we have a network connection
	return true, nil
}

// LinkSpeed attempts to determine link speed, first by checking if any tools
// exist that can return the speed (ethtool for now). If no tools are found,
// fall back to /sys/class/net speed file, if it exists.
//
// The return value is in the format of "<int>MB/s"
//
// LinkSpeed returns an empty string if no tools or sys file are found
func (f *NetworkFingerprint) linkSpeed(device string) int {
	// Use LookPath to find the ethtool in the systems $PATH
	// If it's not found or otherwise errors, LookPath returns and empty string
	// and an error we can ignore for our purposes
	ethtoolPath, _ := exec.LookPath("ethtool")
	if ethtoolPath != "" {
		if speed := f.linkSpeedEthtool(ethtoolPath, device); speed > 0 {
			return speed
		}
	}
	f.logger.Printf("[WARN] fingerprint.network: Ethtool not found, checking /sys/net speed file")

	// Fall back on checking a system file for link speed.
	return f.linkSpeedSys(device)
}

// linkSpeedSys parses the information stored in the sys diretory for the
// default device. This method retuns an empty string if the file is not found
// or cannot be read
func (f *NetworkFingerprint) linkSpeedSys(device string) int {
	path := fmt.Sprintf("/sys/class/net/%s/speed", device)
	_, err := os.Stat(path)
	if err != nil {
		f.logger.Printf("[WARN] fingerprint.network: Error getting information about net speed")
		return 0
	}

	// Read contents of the device/speed file
	content, err := ioutil.ReadFile(path)
	if err == nil {
		lines := strings.Split(string(content), "\n")
		// convert to MB/s
		mbs, err := strconv.Atoi(lines[0])
		if err != nil {
			f.logger.Println("[WARN] fingerprint.network: Enable to parse ethtool output")
			return 0
		}

		if mbs > 0 {
			return mbs
		}
	}
	return 0
}

// linkSpeedEthtool uses the ethtool installed on the node to gather link speed
// information. It executes the command on the device specified and parses
// out the speed. The expected format is Mbps and converted to MB/s
// Returns an empty string there is an error in parsing or executing ethtool
func (f *NetworkFingerprint) linkSpeedEthtool(path, device string) int {
	outBytes, err := exec.Command(path, device).Output()
	if err == nil {
		output := strings.TrimSpace(string(outBytes))
		re := regexp.MustCompile("Speed: [0-9]+[a-zA-Z]+/s")
		m := re.FindString(output)
		if m == "" {
			// no matches found, output may be in a different format
			f.logger.Println("[WARN] fingerprint.network: Ethtool output did not match regex")
			return 0
		}

		// Split and trim the Mb/s unit from the string output
		args := strings.Split(m, ": ")
		raw := strings.TrimSuffix(args[1], "Mb/s")

		// convert to MB/s
		mbs, err := strconv.Atoi(raw)
		if err != nil {
			f.logger.Println("[WARN] fingerprint.network: Unable to parse ethtool output")
			return 0
		}

		if mbs > 0 {
			return mbs
		}
	}
	f.logger.Printf("[ERR] fingerprint.network: Error calling ethtool (%s): %s", path, err)
	return 0
}

// ifConfig returns the IP Address for this node according to ifConfig, for the
// specified device.
func (f *NetworkFingerprint) ifConfig(device string) string {
	ifConfigPath, _ := exec.LookPath("ifconfig")
	if ifConfigPath != "" {
		outBytes, err := exec.Command(ifConfigPath, device).Output()
		if err == nil {
			// Parse out the IP address returned from ifconfig for this device
			// Tested on Ubuntu, the matching part of ifconfig output for eth0 is like
			// so:
			//   inet addr:10.0.2.15  Bcast:10.0.2.255  Mask:255.255.255.0
			// For OS X and en0, we have:
			//  inet 192.168.0.7 netmask 0xffffff00 broadcast 192.168.0.255
			output := strings.TrimSpace(string(outBytes))

			// re is a regular expression, which can vary based on the OS
			var re *regexp.Regexp

			if "darwin" == runtime.GOOS {
				re = regexp.MustCompile("inet [0-9].+")
			} else {
				re = regexp.MustCompile("inet addr:[0-9].+")
			}
			args := strings.Split(re.FindString(output), " ")

			var ip string
			if len(args) > 1 {
				ip = strings.TrimPrefix(args[1], "addr:")
			}

			// validate what we've sliced out is a valid IP
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
		f.logger.Printf("[ERR] fingerprint.network: Error calling ifconfig (%s): %s", ifConfigPath, err)
		return ""
	}

	f.logger.Println("[WARN] fingerprint.network: Ethtool not found")
	return ""
}
