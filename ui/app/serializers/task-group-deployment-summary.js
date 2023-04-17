/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
