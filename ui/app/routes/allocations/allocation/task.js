/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-controller-access-in-routes */
import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import EmberError from '@ember/error';

export default class TaskRoute extends Route {
  @service store;

  model({ name }) {
    const allocation = this.modelFor('allocations.allocation');

    // If there is no allocation, then there is no task.
    // Let the allocation route handle the 404 error.
    if (!allocation) return;

    const task = allocation.get('states').findBy('name', name);

    if (!task) {
      const err = new EmberError(
        `Task ${name} not found for allocation ${allocation.get('id')}`
      );
      err.code = '404';
      this.controllerFor('application').set('error', err);
    }

    return task;
  }
}
