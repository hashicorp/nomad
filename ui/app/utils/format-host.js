/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import isIp from 'is-ip';

export default function formatHost(address, port) {
  if (!address || !port) {
    return undefined;
  }

  if (isIp.v6(address)) {
    return `[${address}]:${port}`;
  } else {
    return `${address}:${port}`;
  }
}
