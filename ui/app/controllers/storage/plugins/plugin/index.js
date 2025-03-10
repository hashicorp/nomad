/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { action, computed } from '@ember/object';

export default class IndexController extends Controller {
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
    this.transitionToRoute('allocations.allocation', allocation.id);
  }
}
