/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import EmberError from '@ember/error';
import { resolve, all } from 'rsvp';
import {
  watchRecord,
  watchRelationship,
} from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyError from 'nomad-ui/utils/notify-error';
import { inject as service } from '@ember/service';

export default class TaskGroupRoute extends Route.extend(WithWatchers) {
  @service store;

  model({ name }) {
    const job = this.modelFor('jobs.job');

    // If there is no job, then there is no task group.
    // Let the job route handle the 404.
    if (!job) return;

    // If the job is a partial (from the list request) it won't have task
    // groups. Reload the job to ensure task groups are present.
    const reload = job.get('isPartial') ? job.reload() : resolve();
    return reload
      .then(() => {
        const taskGroup = job.get('taskGroups').findBy('name', name);
        if (!taskGroup) {
          const err = new EmberError(
            `Task group ${name} for job ${job.get('name')} not found`
          );
          err.code = '404';
          throw err;
        }

        // Refresh job allocations before-hand (so page sort works on load)
        return all([
          job.hasMany('allocations').reload(),
          job.get('scaleState'),
        ]).then(() => taskGroup);
      })
      .catch(notifyError(this));
  }

  startWatchers(controller, model) {
    if (model) {
      const job = model.get('job');
      controller.set('watchers', {
        job: this.watchJob.perform(job),
        summary: this.watchSummary.perform(job.get('summary')),
        scale: this.watchScale.perform(job.get('scaleState')),
        allocations: this.watchAllocations.perform(job),
        latestDeployment:
          job.get('supportsDeployments') &&
          this.watchLatestDeployment.perform(job),
      });
    }
  }

  @watchRecord('job') watchJob;
  @watchRecord('job-summary') watchSummary;
  @watchRecord('job-scale') watchScale;
  @watchRelationship('allocations') watchAllocations;
  @watchRelationship('latestDeployment') watchLatestDeployment;

  @collect(
    'watchJob',
    'watchSummary',
    'watchScale',
    'watchAllocations',
    'watchLatestDeployment'
  )
  watchers;
}
