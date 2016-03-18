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
func IpOfDevice(name string) (net.IP, error) {
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
	for _, addr := range addrs {
		switch v := (addr).(type) {
		case *net.IPNet:
			continue
		case *net.IPAddr:
			return v.IP, nil
		}
	}
	return nil, fmt.Errorf("no ips were detected on the interface: %v", name)
}
