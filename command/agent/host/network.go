// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package host

import (
	"fmt"

	sockaddr "github.com/hashicorp/go-sockaddr"
)

// network uses go-sockaddr to capture our view of the network
// on error, return the text of the error
func network() (output []map[string]string) {
	ifaddrs, err := sockaddr.GetAllInterfaces()
	if err != nil {
		output = append(output, map[string]string{"error": err.Error()})
		return output
	}

	for _, inf := range ifaddrs {
		output = append(output, dumpSockAddr(inf.SockAddr))
	}

	return output
}

// dumpSockAddr is adapted from
// https://github.com/hashicorp/go-sockaddr/blob/c7188e74f6acae5a989bdc959aa779f8b9f42faf/cmd/sockaddr/command/dump.go#L144-L244
func dumpSockAddr(sa sockaddr.SockAddr) map[string]string {
	output := make(map[string]string)

	// Attributes for all SockAddr types
	for _, attr := range sockaddr.SockAddrAttrs() {
		output[string(attr)] = sockaddr.SockAddrAttr(sa, attr)
	}

	// Attributes for all IP types (both IPv4 and IPv6)
	if sa.Type()&sockaddr.TypeIP != 0 {
		ip := *sockaddr.ToIPAddr(sa)
		for _, attr := range sockaddr.IPAttrs() {
			output[string(attr)] = sockaddr.IPAddrAttr(ip, attr)
		}
	}

	if sa.Type() == sockaddr.TypeIPv4 {
		ipv4 := *sockaddr.ToIPv4Addr(sa)
		for _, attr := range sockaddr.IPv4Attrs() {
			output[string(attr)] = sockaddr.IPv4AddrAttr(ipv4, attr)
		}
	}

	if sa.Type() == sockaddr.TypeIPv6 {
		ipv6 := *sockaddr.ToIPv6Addr(sa)
		for _, attr := range sockaddr.IPv6Attrs() {
			output[string(attr)] = sockaddr.IPv6AddrAttr(ipv6, attr)
		}
	}

	if sa.Type() == sockaddr.TypeUnix {
		us := *sockaddr.ToUnixSock(sa)
		for _, attr := range sockaddr.UnixSockAttrs() {
			output[string(attr)] = sockaddr.UnixSockAttr(us, attr)
		}
	}

	// Developer-focused arguments
	{
		arg1, arg2 := sa.DialPacketArgs()
		output["DialPacket"] = fmt.Sprintf("%+q %+q", arg1, arg2)
	}
	{
		arg1, arg2 := sa.DialStreamArgs()
		output["DialStream"] = fmt.Sprintf("%+q %+q", arg1, arg2)
	}
	{
		arg1, arg2 := sa.ListenPacketArgs()
		output["ListenPacket"] = fmt.Sprintf("%+q %+q", arg1, arg2)
	}
	{
		arg1, arg2 := sa.ListenStreamArgs()
		output["ListenStream"] = fmt.Sprintf("%+q %+q", arg1, arg2)
	}

	return output
}
