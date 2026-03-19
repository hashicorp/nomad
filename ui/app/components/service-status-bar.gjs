/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import DistributionBar from 'nomad-ui/components/distribution-bar';

export default class ServiceStatusBar extends Component {
  get serviceStatus() {
    return this.args.status;
  }

  get data() {
    if (!this.serviceStatus) {
      return [];
    }

    const pending = this.serviceStatus.pending || 0;
    const failing = this.serviceStatus.failure || 0;
    const success = this.serviceStatus.success || 0;

    const [grey, red, green] = ['queued', 'failed', 'running'];

    return [
      {
        label: 'Pending',
        value: pending,
        className: grey,
      },
      {
        label: 'Failing',
        value: failing,
        className: red,
      },
      {
        label: 'Success',
        value: success,
        className: green,
      },
    ];
  }

  <template>
    <DistributionBar
      @data={{this.data}}
      @isNarrow={{@isNarrow}}
      @onSliceClick={{@onSliceClick}}
      data-test-service-status-bar
      ...attributes
    />
  </template>
}
