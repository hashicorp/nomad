/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';
import { computed } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class Service extends Fragment {
  @attr('string') name;
  @attr('string') portLabel;
  @attr() tags;
  @attr() canary_tags;
  @attr('string') onUpdate;
  @attr('string') provider;
  @fragment('consul-connect') connect;
  @attr() groupName;
  @attr() taskName;
  get refID() {
    return `${this.groupName || this.taskName}-${this.name}`;
  }
  @attr({ defaultValue: () => [] }) healthChecks;

  @computed('healthChecks.[]')
  get mostRecentChecks() {
    // Get unique check names, then get the most recent one
    return this.get('healthChecks')
      .mapBy('Check')
      .uniq()
      .map((name) => {
        return this.get('healthChecks')
          .sortBy('Timestamp')
          .reverse()
          .find((x) => x.Check === name);
      })
      .sortBy('Check');
  }

  @computed('mostRecentChecks.[]')
  get mostRecentCheckStatus() {
    // Get unique check names, then get the most recent one
    return this.get('mostRecentChecks')
      .mapBy('Status')
      .reduce((acc, curr) => {
        acc[curr] = (acc[curr] || 0) + 1;
        return acc;
      }, {});
  }
}
