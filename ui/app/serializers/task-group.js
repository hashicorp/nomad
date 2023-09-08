/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { copy } from 'ember-copy';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class TaskGroup extends ApplicationSerializer {
  arrayNullOverrides = ['Services'];
  mapToArray = ['Volumes'];

  normalize(typeHash, hash) {
    if (hash.Services) {
      hash.Services.forEach((service) => {
        service.GroupName = hash.Name;
      });
    }
    // Provide EphemeralDisk to each task
    hash.Tasks.forEach((task) => {
      task.EphemeralDisk = copy(hash.EphemeralDisk);
    });

    hash.ReservedEphemeralDisk = hash.EphemeralDisk.SizeMB;

    return super.normalize(typeHash, hash);
  }
}
