// +build linux darwin

package fingerprint

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
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

// NewNetworkFingerprinter returns a new NetworkFingerprinter with the given
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
	// User-defined override for the default interface
	if cfg.NetworkInterface != "" {
		defaultDevice = cfg.NetworkInterface
	}

	newNetwork.Device = defaultDevice

	if ip := f.ipAddress(defaultDevice); ip != "" {
		node.Attributes["network.ip-address"] = ip
		newNetwork.IP = ip
		newNetwork.CIDR = newNetwork.IP + "/32"
	}

	if throughput := f.linkSpeed(defaultDevice); throughput > 0 {
		newNetwork.MBits = throughput
	}

	if node.Resources == nil {
		node.Resources = &structs.Resources{}
	}

	node.Resources.Networks = append(node.Resources.Networks, newNetwork)

	// return true, because we have a network connection
	return true, nil
}

// linkSpeed returns link speed in Mb/s, or 0 when unable to determine it.
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

	// Fall back on checking a system file for link speed.
	return f.linkSpeedSys(device)
}

// linkSpeedSys parses link speed in Mb/s from /sys.
func (f *NetworkFingerprint) linkSpeedSys(device string) int {
	path := fmt.Sprintf("/sys/class/net/%s/speed", device)

	// Read contents of the device/speed file
	content, err := ioutil.ReadFile(path)
	if err != nil {
		f.logger.Printf("[WARN] fingerprint.network: Unable to read link speed from %s", path)
		return 0
	}

	lines := strings.Split(string(content), "\n")
	mbs, err := strconv.Atoi(lines[0])
	if err != nil || mbs <= 0 {
		f.logger.Printf("[WARN] fingerprint.network: Unable to parse link speed from %s", path)
		return 0
	}

	return mbs
}

// linkSpeedEthtool determines link speed in Mb/s with 'ethtool'.
func (f *NetworkFingerprint) linkSpeedEthtool(path, device string) int {
	outBytes, err := exec.Command(path, device).Output()
	if err != nil {
		f.logger.Printf("[WARN] fingerprint.network: Error calling ethtool (%s %s): %v", path, device, err)
		return 0
	}

	output := strings.TrimSpace(string(outBytes))
	re := regexp.MustCompile("Speed: [0-9]+[a-zA-Z]+/s")
	m := re.FindString(output)
	if m == "" {
		// no matches found, output may be in a different format
		f.logger.Printf("[WARN] fingerprint.network: Unable to parse Speed in output of '%s %s'", path, device)
		return 0
	}

	// Split and trim the Mb/s unit from the string output
	args := strings.Split(m, ": ")
	raw := strings.TrimSuffix(args[1], "Mb/s")

	// convert to Mb/s
	mbs, err := strconv.Atoi(raw)
	if err != nil || mbs <= 0 {
		f.logger.Printf("[WARN] fingerprint.network: Unable to parse Mb/s in output of '%s %s'", path, device)
		return 0
	}

	return mbs
}

// ipAddress returns the first IPv4 address on the configured default interface
// Tries Golang native functions and falls back onto ifconfig
func (f *NetworkFingerprint) ipAddress(device string) string {
	if ip, err := f.nativeIpAddress(device); err == nil {
		return ip
	}

	return f.ifConfig(device)
}

func (f *NetworkFingerprint) nativeIpAddress(device string) (string, error) {
	// Find IP address on configured interface
	var ip string
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", errors.New("could not retrieve interface list")
	}

	// TODO: should we handle IPv6 here? How do we determine precedence?
	for _, i := range ifaces {
		if i.Name != device {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			return "", errors.New("could not retrieve interface IP addresses")
		}

		for _, a := range addrs {
			switch v := a.(type) {
			case *net.IPNet:
				if v.IP.To4() != nil {
					ip = v.IP.String()
				}
			case *net.IPAddr:
				if v.IP.To4() != nil {
					ip = v.IP.String()
				}
			}
		}
	}

	if net.ParseIP(ip) == nil {
		return "", errors.New(fmt.Sprintf("could not parse IP address `%s`", ip))
	}

	return ip, nil
}

// ifConfig returns the IP Address for this node according to ifConfig, for the
// specified device.
func (f *NetworkFingerprint) ifConfig(device string) string {
	ifConfigPath, _ := exec.LookPath("ifconfig")
	if ifConfigPath == "" {
		f.logger.Println("[WARN] fingerprint.network: ifconfig not found")
		return ""
	}

	outBytes, err := exec.Command(ifConfigPath, device).Output()
	if err != nil {
		f.logger.Printf("[WARN] fingerprint.network: Error calling ifconfig (%s %s): %v", ifConfigPath, device, err)
		return ""
	}

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
	if net.ParseIP(ip) == nil {
		f.logger.Printf("[WARN] fingerprint.network: Unable to parse IP in output of '%s %s'", ifConfigPath, device)
		return ""
	}

	return ip
}
