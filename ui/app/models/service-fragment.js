/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { compare } from '@ember/utils';
import { get } from '@ember/object';
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
    return [...[...new Set(this.get('healthChecks')
      .map(item => get(item, 'Check')))]
      .map((name) => {
        return [...this.get('healthChecks')]
          .sort((a, b) => compare(get(a, 'Timestamp'), get(b, 'Timestamp')))
          .reverse()
          .find((x) => x.Check === name);
      })]
      .sort((a, b) => compare(get(a, 'Check'), get(b, 'Check')));
  }

  @computed('mostRecentChecks.[]')
  get mostRecentCheckStatus() {
    // Get unique check names, then get the most recent one
    return this.get('mostRecentChecks')
      .map(item => get(item, 'Status'))
      .reduce((acc, curr) => {
        acc[curr] = (acc[curr] || 0) + 1;
        return acc;
      }, {});
  }
}
