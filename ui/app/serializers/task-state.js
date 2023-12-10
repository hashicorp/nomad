/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class TaskState extends ApplicationSerializer {
  normalize(typeHash, hash) {
    // TODO API: finishedAt is always marshaled as a date even when unset.
    // To simplify things, unset it here when it's the empty date value.
    if (hash.FinishedAt === '0001-01-01T00:00:00Z') {
      hash.FinishedAt = null;
    }

    return super.normalize(typeHash, hash);
  }
}
