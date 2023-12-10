/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class JobStatusAllocationStatusBlockComponent extends Component {
  get countToShow() {
    const restWidth = 50;
    const restGap = 10;
    let cts = Math.floor((this.args.width - (restWidth + restGap)) / 42);
    // Either show 3+ or show only a single/remaining box
    return cts > 3 ? cts : 0;
  }

  get remaining() {
    return this.args.count - this.countToShow;
  }
}
