/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import isIp from 'is-ip';
import classic from 'ember-classic-decorator';

@classic
export default class PortSerializer extends ApplicationSerializer {
  attrs = {
    hostIp: 'HostIP',
  };

  normalize(typeHash, hash) {
    const ip = hash.HostIP;

    if (isIp.v6(ip)) {
      hash.HostIP = `[${ip}]`;
    }

    return super.normalize(...arguments);
  }
}
