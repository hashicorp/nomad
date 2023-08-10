/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed as overridable } from 'ember-overridable-computed';
import { inject as service } from '@ember/service';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class RescheduleEventRow extends Component {
  @service store;

  // When given a string, the component will fetch the allocation
  allocationId = null;

  // An allocation can also be provided directly
  @overridable('allocationId', function () {
    if (this.allocationId) {
      return this.store.findRecord('allocation', this.allocationId);
    }

    return null;
  })
  allocation;

  time = null;
  linkToAllocation = true;
  label = '';
}
