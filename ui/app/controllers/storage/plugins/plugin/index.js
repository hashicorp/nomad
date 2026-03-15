/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';

export default class IndexController extends Controller {
  @service router;

  @computed('model.controllers.@each.updateTime')
  get sortedControllers() {
    return this.model.controllers.sortBy('updateTime').reverse();
  }

  @computed('model.nodes.@each.updateTime')
  get sortedNodes() {
    return this.model.nodes.sortBy('updateTime').reverse();
  }

  @action
  gotoAllocation(allocation) {
    this.router.transitionTo('allocations.allocation', allocation.id);
  }
}
