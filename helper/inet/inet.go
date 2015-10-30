package inet

import (
	"fmt"
	"net"
	"strings"
)

func AdvertiseIpFromSubnet(ip string) (string, error) {
	// Return raw IP address if it is not in subnet-form
	if !strings.Contains(ip, "/") {
		return ip, nil
	}

	// Split subnet CIDR IP from the port
	tokens := strings.Split(ip, ":")
	if len(tokens) != 2 {
		return "", fmt.Errorf("port was not given in advertise subnet: `%s`", ip)
	}

	// Parse subnet IP and netmask
	_, subnet, err := net.ParseCIDR(tokens[0])
	if err != nil {
		return "", fmt.Errorf("could not parse `%s` as an IP subnet: %s", tokens[0], err)
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("could not retrieve interface list: %s", err)
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		// Select the first IP address included in our subnet
		for _, a := range addrs {
			switch v := a.(type) {
			case *net.IPNet:
				if subnet.Contains(v.IP) {
					return fmt.Sprintf("%s:%s", v.IP.String(), tokens[1]), nil
				}
			case *net.IPAddr:
				if subnet.Contains(v.IP) {
					return fmt.Sprintf("%s:%s", v.IP.String(), tokens[1]), nil
				}
			}
		}
	}

	return "", fmt.Errorf("found no IP matching `%s` subnet", ip)
}
