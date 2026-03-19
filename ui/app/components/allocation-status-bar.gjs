/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import DistributionBar from 'nomad-ui/components/distribution-bar';

export default class AllocationStatusBar extends Component {
  get container() {
    return this.args.allocationContainer;
  }

  get jobModel() {
    return this.args.job;
  }

  generateLegendLink = (job, status) => {
    if (!job || status === 'queued') return null;

    const namespace = job.namespaceId || job.namespace;
    const queryParams = {
      status: JSON.stringify([status]),
      page: 1,
      search: '',
      sort: 'modifyIndex',
      desc: true,
      client: '',
      taskGroup: '',
      version: '',
      scheduling: '',
      activeTask: null,
    };

    if (namespace && namespace !== 'default') {
      queryParams.namespace = namespace;
    }

    return {
      queryParams,
    };
  };

  get data() {
    if (!this.container) {
      return [];
    }

    const allocs = this.container.getProperties(
      'queuedAllocs',
      'completeAllocs',
      'failedAllocs',
      'runningAllocs',
      'startingAllocs',
      'lostAllocs',
      'unknownAllocs',
    );

    return [
      {
        label: 'Queued',
        value: allocs.queuedAllocs,
        className: 'queued',
        legendLink: this.generateLegendLink(this.jobModel, 'queued'),
      },
      {
        label: 'Starting',
        value: allocs.startingAllocs,
        className: 'starting',
        layers: 2,
        legendLink: this.generateLegendLink(this.jobModel, 'pending'),
      },
      {
        label: 'Running',
        value: allocs.runningAllocs,
        className: 'running',
        legendLink: this.generateLegendLink(this.jobModel, 'running'),
      },
      {
        label: 'Complete',
        value: allocs.completeAllocs,
        className: 'complete',
        legendLink: this.generateLegendLink(this.jobModel, 'complete'),
      },
      {
        label: 'Unknown',
        value: allocs.unknownAllocs,
        className: 'unknown',
        legendLink: this.generateLegendLink(this.jobModel, 'unknown'),
        help: 'Allocation is unknown since its node is disconnected.',
      },
      {
        label: 'Failed',
        value: allocs.failedAllocs,
        className: 'failed',
        legendLink: this.generateLegendLink(this.jobModel, 'failed'),
      },
      {
        label: 'Lost',
        value: allocs.lostAllocs,
        className: 'lost',
        legendLink: this.generateLegendLink(this.jobModel, 'lost'),
      },
    ];
  }

  <template>
    <DistributionBar
      @data={{this.data}}
      @isNarrow={{@isNarrow}}
      @onSliceClick={{@onSliceClick}}
      class={{@class}}
      data-test-allocation-status-bar
      ...attributes
      as |chart|
    >
      {{yield chart}}
    </DistributionBar>
  </template>
}
