/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import DistributionBar from './distribution-bar';

export default class ChildrenStatusBar extends Component {
  get data() {
    if (!this.args.job) {
      return [];
    }

    const children = this.args.job.getProperties(
      'pendingChildren',
      'runningChildren',
      'deadChildren',
    );

    return [
      {
        label: 'Pending',
        value: children.pendingChildren,
        className: 'queued',
      },
      {
        label: 'Running',
        value: children.runningChildren,
        className: 'running',
      },
      { label: 'Dead', value: children.deadChildren, className: 'complete' },
    ];
  }

  <template>
    <DistributionBar
      @data={{this.data}}
      @isNarrow={{@isNarrow}}
      @onSliceClick={{@onSliceClick}}
      data-test-children-status-bar
      ...attributes
      as |chart|
    >
      {{yield chart}}
    </DistributionBar>
  </template>
}
