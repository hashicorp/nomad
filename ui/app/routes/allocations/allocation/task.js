/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-controller-access-in-routes */
import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

export default class TaskRoute extends Route {
  @service store;
  @service router;

  serialize(model) {
    const modelName =
      (typeof model?.get === 'function' ? model.get('name') : undefined) ||
      model?.name;

    let currentName;
    try {
      currentName = this.paramsFor('allocations.allocation.task')?.name;
    } catch {
      currentName = undefined;
    }

    const taskControllerModel = this.controllerFor(
      'allocations.allocation.task',
    )?.model;
    const taskControllerName =
      (typeof taskControllerModel?.get === 'function'
        ? taskControllerModel.get('name')
        : undefined) || taskControllerModel?.name;

    let routeModelName;
    try {
      const routeModel = this.modelFor('allocations.allocation.task');
      routeModelName =
        (typeof routeModel?.get === 'function'
          ? routeModel.get('name')
          : undefined) || routeModel?.name;
    } catch {
      routeModelName = undefined;
    }

    const currentPath = (this.router.currentURL || '').split('?')[0];
    const urlTaskName = currentPath.match(
      /^\/allocations\/[^/]+\/([^/]+)/,
    )?.[1];

    return {
      name:
        modelName ||
        currentName ||
        taskControllerName ||
        routeModelName ||
        urlTaskName,
    };
  }

  async model({ name }) {
    const allocation = this.modelFor('allocations.allocation');

    // If there is no allocation, then there is no task.
    // Let the allocation route handle the 404 error.
    if (!allocation) return;

    const task = allocation.get('states').findBy('name', name);

    if (!task) {
      const err = new Error(
        `Task ${name} not found for allocation ${allocation.get('id')}`,
      );
      err.code = '404';
      this.controllerFor('application').set('error', err);
    }

    // Ensure variable linkage is hydrated before first render so the
    // task stats section can conditionally show the Variables link.
    await task?.task?.getPathLinkedVariable?.();

    return task;
  }
}
