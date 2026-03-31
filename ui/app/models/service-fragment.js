/**
 * Copyright IBM Corp. 2015, 2025
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
    const checks = this.get('healthChecks')
      .map(item => item.Check);
    return [...new Set(checks)].map((name) => {
        return [...this.get('healthChecks')]
          .sort((a, b) => (b.Timestamp || 0) - (a.Timestamp || 0))
          .find((x) => x.Check === name);
      })
      .sort((a, b) => a.Check?.localeCompare(b.Check) || 0);
  }

  @computed('mostRecentChecks.[]')
  get mostRecentCheckStatus() {
    // Get unique check names, then get the most recent one
    return this.get('mostRecentChecks')
      .map(item => item.Status)
      .reduce((acc, curr) => {
        acc[curr] = (acc[curr] || 0) + 1;
        return acc;
      }, {});
  }
}
