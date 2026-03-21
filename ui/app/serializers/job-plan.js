/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import { get } from '@ember/object';

export default class JobPlan extends ApplicationSerializer {
  mapToArray = ['FailedTGAllocs'];

  normalize(typeHash, hash) {
    hash.PreemptionIDs = (get(hash, 'Annotations.PreemptedAllocs') || []).mapBy(
      'ID',
    );
    return super.normalize(...arguments);
  }
}
