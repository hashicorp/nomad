/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import moment from 'moment';
import formatDate from 'nomad-ui/helpers/format-date';
import JobVersion from 'nomad-ui/components/job-version';

export default class JobVersionsStream extends Component {
  get versions() {
    return this.args.versions ?? [];
  }

  get diffs() {
    return this.args.diffs ?? [];
  }

  get verbose() {
    return this.args.verbose ?? true;
  }

  get annotatedVersions() {
    const versions = [...this.versions].sort((a, b) => {
      return (b.submitTime ?? 0) - (a.submitTime ?? 0);
    });

    return versions.map((version, index) => {
      const meta = {};

      if (index === 0) {
        meta.showDate = true;
      } else {
        const previousVersion = versions[index - 1];
        const previousStart = moment(previousVersion.get('submitTime')).startOf(
          'day',
        );
        const currentStart = moment(version.get('submitTime')).startOf('day');
        if (previousStart.diff(currentStart, 'days') > 0) {
          meta.showDate = true;
        }
      }

      const diff = this.diffs[index];
      return { version, meta, diff };
    });
  }

  <template>
    <ol class="timeline">
      {{#each this.annotatedVersions key="version.id" as |record|}}
        {{#if record.meta.showDate}}
          <li data-test-version-time class="timeline-note">
            {{formatDate record.version.submitTime}}
          </li>
        {{/if}}
        <li data-test-version class="timeline-object">
          <JobVersion
            @version={{record.version}}
            @diff={{record.diff}}
            @verbose={{this.verbose}}
            @handleError={{@handleError}}
            @diffsExpanded={{@diffsExpanded}}
          />
        </li>
      {{/each}}
    </ol>
  </template>
}
