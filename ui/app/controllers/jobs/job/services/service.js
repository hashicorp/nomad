/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class JobsJobServicesServiceController extends Controller {
  @service router;
  queryParams = ['level'];

  @action
  gotoAllocation(allocation) {
    this.router.transitionTo('allocations.allocation', allocation.get('id'));
  }
}
