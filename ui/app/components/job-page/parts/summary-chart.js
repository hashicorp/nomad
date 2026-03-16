/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { camelize } from '@ember/string';
import { inject as service } from '@ember/service';

export default class JobPagePartsSummaryChartComponent extends Component {
  @service router;

  @action
  gotoAllocations(status) {
    const namespace = this.args.job.namespaceId || this.args.job.namespace;
    const queryParams = {
      status: JSON.stringify(status),
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

    this.router.transitionTo('jobs.job.allocations', this.args.job, {
      queryParams,
    });
  }

  @action
  onSliceClick(ev, slice) {
    this.gotoAllocations([camelize(slice.label)]);
  }
}
