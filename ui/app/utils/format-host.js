/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { isIPv6 } from 'is-ip';

export default function formatHost(address, port) {
  if (!address || !port) {
    return undefined;
  }

  if (isIPv6(address)) {
    return `[${address}]:${port}`;
  } else {
    return `${address}:${port}`;
  }
}
