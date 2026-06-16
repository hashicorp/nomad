/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { service } from '@ember/service';

export default class LogsRoute extends Route {
  @service abilities;
  model() {
    const task = this.modelFor('allocations.allocation.task')
    // streaming logs can require the node object to be loaded
    // in order for the logUrl() function to correctly use the
    // allocation.node.httpAddr property.
    // An alternative option here is to use just allocation.node
    // as the computed property for logUrl()
    if (this.abilities.can('read client')){
      return task.get('allocation.node').then(() => task)
    }
    return task
  }
}
