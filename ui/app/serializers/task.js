/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class Task extends ApplicationSerializer {
  normalize(typeHash, hash) {
    // Lift the reserved resource numbers out of the Resources object
    const resources = hash.Resources;
    if (resources) {
      hash.ReservedMemory = resources.MemoryMB;
      hash.ReservedMemoryMax = resources.MemoryMaxMB;
      hash.ReservedCPU = resources.CPU;
      hash.ReservedDisk = resources.DiskMB;
      hash.ReservedEphemeralDisk = hash.EphemeralDisk.SizeMB;
    }

    return super.normalize(typeHash, hash);
  }
}
