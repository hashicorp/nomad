/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class TaskGroupDeploymentSummary extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.PlacedCanaryAllocations = hash.PlacedCanaries || [];
    delete hash.PlacedCanaries;
    return super.normalize(typeHash, hash);
  }
}
