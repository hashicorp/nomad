/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import moment from 'moment';
import formatDate from 'nomad-ui/helpers/format-date';
import JobDeployment from 'nomad-ui/components/job-deployment';

export default class JobDeploymentsStream extends Component {
  get deployments() {
    return normalizeCollection(this.args.deployments);
  }

  get sortedDeployments() {
    return [...this.deployments].sort((a, b) => {
      return (b.versionSubmitTime ?? 0) - (a.versionSubmitTime ?? 0);
    });
  }

  get annotatedDeployments() {
    const deployments = this.sortedDeployments;
    return deployments.map((deployment, index) => {
      const meta = {};

      if (index === 0) {
        meta.showDate = true;
      } else {
        const previousDeployment = deployments[index - 1];
        const previousSubmitTime = previousDeployment.get('version.submitTime');
        const submitTime = deployment.get('submitTime');
        if (
          submitTime &&
          previousSubmitTime &&
          moment(previousSubmitTime)
            .startOf('day')
            .diff(moment(submitTime).startOf('day'), 'days') > 0
        ) {
          meta.showDate = true;
        }
      }

      return { deployment, meta };
    });
  }

  <template>
    <ol class="timeline" ...attributes>
      {{#each this.annotatedDeployments key="deployment.id" as |record|}}
        {{#if record.meta.showDate}}
          <li data-test-deployment-time class="timeline-note">
            {{#if record.deployment.version.submitTime}}
              {{formatDate record.deployment.version.submitTime}}
            {{else}}
              Unknown time
            {{/if}}
          </li>
        {{/if}}
        <li data-test-deployment class="timeline-object">
          <JobDeployment @deployment={{record.deployment}} />
        </li>
      {{/each}}
    </ol>
  </template>
}

function normalizeCollection(value) {
  if (!value) {
    return [];
  }

  if (Array.isArray(value)) {
    return [...value];
  }

  if (typeof value.toArray === 'function') {
    return value.toArray();
  }

  if (typeof value[Symbol.iterator] === 'function') {
    return Array.from(value);
  }

  return [];
}
