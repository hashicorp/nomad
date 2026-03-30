/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { service } from '@ember/service';
import { action, computed } from '@ember/object';

export default class IndexController extends Controller {
  @service router;

  @computed('model.controllers.@each.updateTime')
  get sortedControllers() {
    return [...this.model.controllers].sort((a, b) => (b.updateTime || 0) - (a.updateTime || 0));
  }

  @computed('model.nodes.@each.updateTime')
  get sortedNodes() {
    return [...this.model.nodes].sort((a, b) => (b.updateTime || 0) - (a.updateTime || 0));
  }

  get topControllers() {
    return this.sortedControllers.slice(0, 10);
  }

  get topNodes() {
    return this.sortedNodes.slice(0, 10);
  }

  @action
  gotoAllocation(allocation) {
    this.router.transitionTo('allocations.allocation', allocation.id);
  }
}
