package inet

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

const (
	advertiseIp = iota
	advertisePort
)

const (
	address = iota
	cidr
)

func AdvertiseIpFromSubnet(ip string) (string, error) {
	// Return raw IP address if it is not in subnet-form
	if !strings.Contains(ip, "/") {
		return ip, nil
	}

	// Split subnet CIDR IP from the port
	advertiseTokens := strings.Split(ip, ":")
	if len(advertiseTokens) != 2 {
		return "", errors.New(fmt.Sprintf("port was not given in advertise subnet: `%s`", ip))
	}

	// Split subnet IP and CIDR netmask by slash separator
	ipTokens := strings.Split(advertiseTokens[advertiseIp], "/")
	if len(ipTokens) != 2 {
		return "", errors.New(fmt.Sprintf("cannot parse advertise subnet: `%s`", ip))
	}

	// Parse subnet IP and netmask
	subnetIp := net.ParseIP(ipTokens[address])
	if subnetIp == nil {
		return "", errors.New(fmt.Sprintf("advertise subnet address out of range: `%s`", ipTokens[address]))
	}
	subnetCidr, err := strconv.ParseInt(ipTokens[cidr], 10, 32)
	if err != nil {
		return "", errors.New(fmt.Sprintf("cannot parse advertise subnet CIDR: `%s`", ipTokens[cidr]))
	}

	subnet := net.IPNet{IP: subnetIp, Mask: net.CIDRMask(int(subnetCidr), 32)}
	if subnet.Mask == nil {
		return "", errors.New(fmt.Sprintf("advertise subnet CIDR out of range: `%s`", ipTokens[cidr]))
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", errors.New("could not retrieve interface list")
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
					return fmt.Sprintf("%s:%s", v.IP.String(), advertiseTokens[advertisePort]), nil
				}
			case *net.IPAddr:
				if subnet.Contains(v.IP) {
					return fmt.Sprintf("%s:%s", v.IP.String(), advertiseTokens[advertisePort]), nil
				}
			}
		}
	}

	return "", errors.New(fmt.Sprintf("found no IP matching `%s` subnet", ip))
}
