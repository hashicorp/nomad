/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { get } from '@ember/object';
import { LinkTo } from '@ember/routing';
import {
  HdsBadge,
  HdsIcon,
} from '@hashicorp/design-system-components/components';

export default class JobStatusLatestDeployment extends Component {
  get deployment() {
    return get(this.args.job, 'latestDeployment');
  }

  get status() {
    return get(this.deployment, 'status');
  }

  get healthyAllocs() {
    const summaries = get(this.deployment, 'taskGroupSummaries') || [];
    return summaries
      .mapBy('healthyAllocs')
      .reduce((sum, count) => sum + count, 0);
  }

  get desiredTotal() {
    const summaries = get(this.deployment, 'taskGroupSummaries') || [];
    return summaries
      .mapBy('desiredTotal')
      .reduce((sum, count) => sum + count, 0);
  }

  get statusColor() {
    switch (this.status) {
      case 'successful':
        return 'success';
      case 'failed':
        return 'critical';
      default:
        return 'neutral';
    }
  }

  get statusLabel() {
    if (!this.status) {
      return '';
    }

    return `${this.status.charAt(0).toUpperCase()}${this.status.slice(1)}`;
  }

  <template>
    <section class="latest-deployment" ...attributes>
      <LinkTo @route="jobs.job.deployments" @model={{@job}}>
        <h4>
          Latest Deployment
          <HdsIcon @name="arrow-right" @isInline={{true}} />
        </h4>
      </LinkTo>
      <HdsBadge
        @text={{this.statusLabel}}
        @size="small"
        @color={{this.statusColor}}
        @type="filled"
      />
      <p>{{this.healthyAllocs}}/{{this.desiredTotal}} Healthy</p>
    </section>
  </template>
}
