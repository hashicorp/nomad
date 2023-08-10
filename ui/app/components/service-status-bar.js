/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import DistributionBar from './distribution-bar';
import { attributeBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@attributeBindings('data-test-service-status-bar')
export default class ServiceStatusBar extends DistributionBar {
  layoutName = 'components/distribution-bar';

  status = null;

  'data-test-service-status-bar' = true;

  @computed('status.{failure,pending,success}')
  get data() {
    if (!this.status) {
      return [];
    }

    const pending = this.status.pending || 0;
    const failing = this.status.failure || 0;
    const success = this.status.success || 0;

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
}
