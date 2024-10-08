/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import moment from 'moment';
import { classNames, tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('ol')
@classNames('timeline')
export default class JobVersionsStream extends Component {
  @overridable(() => []) versions;

  // Passes through to the job-diff component
  verbose = true;

  diffs = [];

  @computed('versions.[]', 'diffs.[]')
  get annotatedVersions() {
    const versions = this.versions.sortBy('submitTime').reverse();
    return versions.map((version, index) => {
      const meta = {};

      if (index === 0) {
        meta.showDate = true;
      } else {
        const previousVersion = versions.objectAt(index - 1);
        const previousStart = moment(previousVersion.get('submitTime')).startOf(
          'day'
        );
        const currentStart = moment(version.get('submitTime')).startOf('day');
        if (previousStart.diff(currentStart, 'days') > 0) {
          meta.showDate = true;
        }
      }

      const diff = this.diffs.objectAt(index);
      return { version, meta, diff };
    });
  }
}
