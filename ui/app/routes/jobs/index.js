/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import { collect } from '@ember/object/computed';
import { watchAll, watchQuery } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';

export default class IndexRoute extends Route.extend(
  WithWatchers,
  WithForbiddenState
) {
  @service store;

  queryParams = {
    qpNamespace: {
      refreshModel: true,
    },
  };

  async model(params) {
    const jobs = await this.store
      .query('job', { namespace: params.qpNamespace, meta: true })
      .catch(notifyForbidden(this));

    let jobStatuses = null;
    if (jobs && jobs.length) {
      jobStatuses = await this.store
        .query('job-status', {
          jobs: jobs.map((job) => {
            return {
              id: job.plainId,
              namespace: job.belongsTo('namespace').id(),
            };
          }),
        })
        .then((jobStatuses) => {
          // assign each to job
          jobStatuses.forEach((jobStatus) => {
            const job = jobs.findBy('plainId', jobStatus.get('id'));
            job.set('jobStatus', jobStatus);
          });
          return jobStatuses.sortBy('job.id');
        })
        .catch(notifyForbidden(this));
    }

    return RSVP.hash({
      jobs,
      jobStatuses,
      namespaces: this.store.findAll('namespace'),
      nodePools: this.store.findAll('node-pool'),
    });
  }

  startWatchers(controller, model) {
    controller.set('namespacesWatch', this.watchNamespaces.perform());
    controller.set(
      'modelWatch',
      this.watchJobs.perform({ namespace: controller.qpNamespace, meta: true })
    );
    // TODO: This is running on a 1-second throttle / not making use of blocking query by index.
    controller.set(
      'jobStatusWatch',
      this.watchStatuses.perform({
        jobs: model.jobs.map((job) => {
          return {
            id: job.plainId,
            namespace: job.belongsTo('namespace').id(),
          };
        }),
      })
    );
  }

  @watchQuery('job') watchJobs;
  @watchQuery('jobStatus') watchStatuses;
  @watchAll('namespace') watchNamespaces;
  @collect('watchJobs', 'watchNamespaces', 'watchStatuses') watchers;
}
