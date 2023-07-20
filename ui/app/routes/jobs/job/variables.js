/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class JobsJobVariablesRoute extends Route {
  @service can;
  @service router;
  @service store;

  beforeModel() {
    if (this.can.cannot('list variables')) {
      this.router.transitionTo(`/jobs/job`);
    }
  }
  async model() {
    let job = this.modelFor('jobs.job');
    let taskGroups = job.taskGroups;
    // let tasks = taskGroups.map((tg) => tg.tasks).flat();
    let tasks = taskGroups.map((tg) => tg.tasks.toArray()).flat();
    tasks.forEach((task) => {
      if (!task._job) {
        task._job = job;
      }
    });

    let jobVariable = await job.getPathLinkedVariable();
    console.log('job first', jobVariable);
    let groupVariables = await Promise.all(
      taskGroups.map((tg) => tg.getPathLinkedVariable())
    );
    console.log('then gruppes', groupVariables);
    // let jobVariable = await job.pathLinkedVariable;
    let taskVariables = await Promise.all(
      tasks.map((task) => task.getPathLinkedVariable())
    );

    let allJobsVariable;
    try {
      allJobsVariable = await this.store.findRecord('variable', 'nomad/jobs');
      console.log('allJobsVariable', allJobsVariable);
    } catch (error) {
      console.log('allJobsVariable error', error);
    }
    console.log('tasks then', taskVariables);
    // const variables = await this.store.findAll('variable', {
    //   reload: true,
    // });

    const variables = [
      allJobsVariable,
      jobVariable,
      ...groupVariables,
      ...taskVariables,
    ].compact();
    return { variables, job: this.modelFor('jobs.job') };
  }
}
