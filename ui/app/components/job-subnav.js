/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Component from '@glimmer/component';

export default class JobSubnav extends Component {
  @service can;
  @service keyboard;

  get shouldRenderClientsTab() {
    const { job } = this.args;
    return (
      job?.hasClientStatus && !job?.hasChildren && this.can.can('read client')
    );
  }

  // Periodic and Parameterized jobs "parents" are not jobs unto themselves, but more like summaries.
  // They should not have tabs for allocations, evaluations, etc.
  // but their child jobs, and other job types generally, should.
  get shouldHideNonParentTabs() {
    return this.args.job?.hasChildren;
  }
}
