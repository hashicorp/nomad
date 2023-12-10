/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class ResourcesSerializer extends ApplicationSerializer {
  arrayNullOverrides = ['Ports', 'Networks'];

  normalize(typeHash, hash) {
    hash.Cpu = hash.Cpu && hash.Cpu.CpuShares;
    hash.Memory = hash.Memory && hash.Memory.MemoryMB;
    hash.Disk = hash.Disk && hash.Disk.DiskMB;

    // Networks for ReservedResources is different than for Resources.
    // This smooths over the differences, but doesn't actually support
    // anything in the ReservedResources.Networks object, since we don't
    // use any of it in the UI.
    if (!(hash.Networks instanceof Array)) {
      hash.Networks = [];
    }

    return super.normalize(...arguments);
  }
}
