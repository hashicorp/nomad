/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
// eslint-disable-next-line no-unused-vars
import JobModel from '../../../models/job';
import { A } from '@ember/array';

export default class JobsJobVariablesRoute extends Route {
  @service can;
  @service router;
  @service store;

  beforeModel() {
    if (this.can.cannot('list variables')) {
      this.router.transitionTo(`/jobs`);
    }
  }
  async model() {
    /** @type {JobModel} */
    let job = this.modelFor('jobs.job');
    let taskGroups = job.taskGroups;
    let tasks = taskGroups.map((tg) => tg.tasks.toArray()).flat();

    let jobVariable = await job.getPathLinkedVariable();
    let groupVariables = await Promise.all(
      taskGroups.map((tg) => tg.getPathLinkedVariable())
    );
    let taskVariables = await Promise.all(
      tasks.map((task) => task.getPathLinkedVariable())
    );

    let allJobsVariable;
    try {
      allJobsVariable = await this.store.findRecord('variable', 'nomad/yobs');
    } catch (e) {
      if (e.errors.findBy('status', 404)) {
        allJobsVariable = null;
      }
    }

    const variables = A([
      allJobsVariable,
      jobVariable,
      ...groupVariables,
      ...taskVariables,
    ]).compact();

    return { variables, job: this.modelFor('jobs.job') };
  }
}
