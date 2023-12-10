/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';
import classic from 'ember-classic-decorator';
import { attributeBindings } from '@ember-decorators/component';

@classic
@attributeBindings('data-test-children-status-bar')
export default class ChildrenStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  job = null;

  'data-test-children-status-bar' = true;

  @computed('job.{pendingChildren,runningChildren,deadChildren}')
  get data() {
    if (!this.job) {
      return [];
    }

    const children = this.job.getProperties(
      'pendingChildren',
      'runningChildren',
      'deadChildren'
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
}
