/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class JobScale extends ApplicationSerializer {
  mapToArray = [{ beforeName: 'TaskGroups', afterName: 'TaskGroupScales' }];

  normalize(modelClass, hash) {
    hash.PlainJobId = hash.JobID;
    hash.ID = JSON.stringify([hash.JobID, hash.Namespace || 'default']);
    hash.JobID = hash.ID;

    return super.normalize(modelClass, hash);
  }
}
