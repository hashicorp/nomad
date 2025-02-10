/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { fragment } from 'ember-data-model-fragments/attributes';
import { attr, belongsTo } from '@ember-data/model';

export default class JobVersion extends Model {
  @belongsTo('job') job;
  @attr('boolean') stable;
  @attr('date') submitTime;
  @attr('number') number;
  @attr() diff;
  @fragment('version-tag') versionTag;

  revertTo() {
    return this.store.adapterFor('job-version').revertTo(this);
  }
}
