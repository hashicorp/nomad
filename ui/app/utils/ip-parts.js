/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// Splits any IP address into an address and a port
export default function ipParts(ip) {
  const parts = ip ? ip.split(':') : [];
  if (parts.length === 0) {
    // ipv4, no port
    return { address: ip, port: undefined };
  } else if (parts.length === 2) {
    // ipv4, with port
    return { address: parts[0], port: parts[1] };
  } else if (ip.startsWith('[')) {
    // ipv6, with port
    return {
      address: parts.slice(0, parts.length - 1).join(':'),
      port: parts[parts.length - 1],
    };
  } else {
    // ipv6, no port
    return { address: ip, port: undefined };
  }
}
