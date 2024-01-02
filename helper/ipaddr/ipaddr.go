// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ipaddr

// IsAny checks if the given IP address is an IPv4 or IPv6 ANY address.
func IsAny(ip string) bool {
	return isAnyV4(ip) || isAnyV6(ip)
}

func isAnyV4(ip string) bool { return ip == "0.0.0.0" }

func isAnyV6(ip string) bool { return ip == "::" || ip == "[::]" }
