package agent

import (
	"fmt"
	"net"
)

// IpOfDevice returns a routable ip addr of a device
func ipOfDevice(name string) (net.IP, error) {
	intf, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := intf.Addrs()
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no ips were detected on the interface: %v", name)
	}

	// Iterating through the IPs configured for that device and returning the
	// the first ipv4 address configured. If no ipv4 addresses are configured,
	// we return the first ipv6 addr if any ipv6 addr is configured.
	var ipv6Addrs []net.IP
	for _, addr := range addrs {
		var ip net.IP
		switch v := (addr).(type) {
		case *net.IPNet:
			ip = v.IP
			if ip.To4() != nil {
				return ip, nil
			}
			if ip.To16() != nil {
				ipv6Addrs = append(ipv6Addrs, ip)
				continue
			}
		case *net.IPAddr:
			continue
		}
	}
	if len(ipv6Addrs) > 0 {
		return ipv6Addrs[0], nil
	}
	return nil, fmt.Errorf("no ips were detected on the interface: %v", name)
}
