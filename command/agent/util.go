package agent

import (
	"fmt"
	"math/rand"
	"net"
	"time"
)

// Returns a random stagger interval between 0 and the duration
func randomStagger(intv time.Duration) time.Duration {
	return time.Duration(uint64(rand.Int63()) % uint64(intv))
}

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
	var ipv4Addrs []net.IP
	var ipv6Addrs []net.IP
	for _, addr := range addrs {
		var ip net.IP
		switch v := (addr).(type) {
		case *net.IPNet:
			continue
		case *net.IPAddr:
			ip = v.IP
			if ip.To4() != nil {
				ipv4Addrs = append(ipv4Addrs, ip)
				continue
			}
			if ip.To16() != nil {
				ipv6Addrs = append(ipv6Addrs, ip)
				continue
			}
		}
	}
	if len(ipv4Addrs) > 0 {
		return ipv4Addrs[0], nil
	}
	if len(ipv6Addrs) > 0 {
		return ipv6Addrs[0], nil
	}
	return nil, fmt.Errorf("no ips were detected on the interface: %v", name)
}
