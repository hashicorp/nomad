/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { alias } from '@ember/object/computed';

export default class JobStatusLatestDeploymentComponent extends Component {
  @alias('args.job.latestDeployment') deployment;
  @alias('deployment.status') status;

  get healthyAllocs() {
    return this.deployment
      .get('taskGroupSummaries')
      .mapBy('healthyAllocs')
      .reduce((sum, count) => sum + count, 0);
  }
  get desiredTotal() {
    return this.deployment
      .get('taskGroupSummaries')
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
}
