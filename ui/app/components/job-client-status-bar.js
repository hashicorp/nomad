/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';
import { attributeBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@attributeBindings('data-test-job-client-status-bar')
export default class JobClientStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  'data-test-job-client-status-bar' = true;
  job = null;
  jobClientStatus = null;

  legendQueryParamsForStatus(status) {
    const namespace = this.job?.namespaceId || this.job?.namespace?.get?.('id');
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
  }

  @computed('job.namespace', 'jobClientStatus.byStatus')
  get data() {
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
    } = this.jobClientStatus.byStatus;

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
}
