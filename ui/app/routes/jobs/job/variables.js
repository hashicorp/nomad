/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
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

    let jobVariablePromise = job.getPathLinkedVariable();
    let groupVariablesPromises = taskGroups.map((tg) =>
      tg.getPathLinkedVariable()
    );
    let taskVariablesPromises = tasks.map((task) =>
      task.getPathLinkedVariable()
    );

    let allJobsVariablePromise = this.store
      .query('variable', {
        path: 'nomad/jobs',
      })
      .then((variables) => {
        return variables.findBy('path', 'nomad/jobs');
      })
      .catch((e) => {
        if (e.errors?.findBy('status', 404)) {
          return null;
        }
        throw e;
      });

    const variables = A(
      await Promise.all([
        allJobsVariablePromise,
        jobVariablePromise,
        ...groupVariablesPromises,
        ...taskVariablesPromises,
      ])
    ).compact();

    return { variables, job: this.modelFor('jobs.job') };
  }
}
