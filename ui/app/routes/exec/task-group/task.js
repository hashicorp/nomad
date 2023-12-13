/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-controller-access-in-routes */
import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

export default class TaskRoute extends Route {
  @service store;

  model({ task_name }) {
    const allocationQueryParam = this.paramsFor('exec').allocation;
    const taskGroupName = this.paramsFor('exec.task-group').task_group_name;

    return {
      allocationShortId: allocationQueryParam,
      taskName: task_name,
      taskGroupName,
    };
  }

  setupController(controller, { allocationShortId, taskGroupName, taskName }) {
    this.controllerFor('exec').send('setTaskProperties', {
      allocationShortId,
      taskName,
      taskGroupName,
    });

    super.setupController(...arguments);
  }
}
