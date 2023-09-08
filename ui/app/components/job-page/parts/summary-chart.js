/**
 * Copyright (c) HashiCorp, Inc.
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
    this.router.transitionTo('jobs.job.allocations', this.args.job, {
      queryParams: {
        status: JSON.stringify(status),
        namespace: this.args.job.get('namespace.name'),
      },
    });
  }

  @action
  onSliceClick(ev, slice) {
    this.gotoAllocations([camelize(slice.label)]);
  }
}
