/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import DistributionBar from './distribution-bar';

export default class JobClientStatusBar extends Component {
  legendQueryParamsForStatus = (status) => {
    const namespace =
      this.args.job?.namespaceId || this.args.job?.namespace?.get?.('id');
    const queryParams = {
      status: JSON.stringify([status]),
      page: 1,
      search: '',
      dc: '',
      clientclass: '',
    };

    if (namespace && namespace !== 'default') {
      queryParams.namespace = namespace;
    }

    return queryParams;
  };

  get data() {
    const byStatus = this.args.jobClientStatus?.byStatus;
    if (!byStatus) {
      return [];
    }

    const {
      queued,
      starting,
      running,
      complete,
      degraded,
      failed,
      lost,
      notScheduled,
      unknown,
    } = byStatus;

    return [
      {
        label: 'Queued',
        value: queued.length,
        className: 'queued',
        legendLink: {
          queryParams: this.legendQueryParamsForStatus('queued'),
        },
      },
      {
        label: 'Starting',
        value: starting.length,
        className: 'starting',
        legendLink: {
          queryParams: this.legendQueryParamsForStatus('starting'),
        },
        layers: 2,
      },
      {
        label: 'Running',
        value: running.length,
        className: 'running',
        legendLink: {
          queryParams: this.legendQueryParamsForStatus('running'),
        },
      },
      {
        label: 'Complete',
        value: complete.length,
        className: 'complete',
        legendLink: {
          queryParams: this.legendQueryParamsForStatus('complete'),
        },
      },
      {
        label: 'Unknown',
        value: unknown.length,
        className: 'unknown',
        legendLink: {
          queryParams: this.legendQueryParamsForStatus('unknown'),
        },
        help: 'Some allocations for this job were degraded or lost connectivity.',
      },
      {
        label: 'Degraded',
        value: degraded.length,
        className: 'degraded',
        legendLink: {
          queryParams: this.legendQueryParamsForStatus('degraded'),
        },
        help: 'Some allocations for this job were not successfull or did not run.',
      },
      {
        label: 'Failed',
        value: failed.length,
        className: 'failed',
        legendLink: {
          queryParams: this.legendQueryParamsForStatus('failed'),
        },
      },
      {
        label: 'Lost',
        value: lost.length,
        className: 'lost',
        legendLink: {
          queryParams: this.legendQueryParamsForStatus('lost'),
        },
      },
      {
        label: 'Not Scheduled',
        value: notScheduled.length,
        className: 'not-scheduled',
        legendLink: {
          queryParams: this.legendQueryParamsForStatus('notScheduled'),
        },
        help: 'No allocations for this job were scheduled into these clients.',
      },
    ];
  }

  <template>
    <DistributionBar
      @data={{this.data}}
      @isNarrow={{@isNarrow}}
      @onSliceClick={{@onSliceClick}}
      data-test-job-client-status-bar
      ...attributes
      as |chart|
    >
      {{yield chart}}
    </DistributionBar>
  </template>
}
