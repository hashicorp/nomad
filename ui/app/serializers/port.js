/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import { isIPv6 } from 'is-ip';

export default class PortSerializer extends ApplicationSerializer {
  attrs = {
    hostIp: 'HostIP',
  };

  normalize(typeHash, hash) {
    const ip = hash.HostIP;

    if (isIPv6(ip)) {
      hash.HostIP = `[${ip}]`;
    }

    return super.normalize(...arguments);
  }
}
