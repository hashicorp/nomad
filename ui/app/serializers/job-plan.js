/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import { get } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class JobPlan extends ApplicationSerializer {
  mapToArray = ['FailedTGAllocs'];

  normalize(typeHash, hash) {
    hash.PreemptionIDs = (get(hash, 'Annotations.PreemptedAllocs') || []).mapBy(
      'ID'
    );
    return super.normalize(...arguments);
  }
}
