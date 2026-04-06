/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import { compare } from '@ember/utils';
import Controller from '@ember/controller';
import { service } from '@ember/service';
import { action, computed } from '@ember/object';

export default class IndexController extends Controller {
  @service router;

  @computed('model.controllers.@each.updateTime')
  get sortedControllers() {
    return [...this.model.controllers].sort((a, b) => compare(get(a, 'updateTime'), get(b, 'updateTime'))).reverse();
  }

  @computed('model.nodes.@each.updateTime')
  get sortedNodes() {
    return [...this.model.nodes].sort((a, b) => compare(get(a, 'updateTime'), get(b, 'updateTime'))).reverse();
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
