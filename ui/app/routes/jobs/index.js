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
      .query('job', {
        namespace: params.qpNamespace,
        meta: true,
        queryType: 'initialize',
      })
      .catch(notifyForbidden(this));

    return RSVP.hash({
      jobs,
      namespaces: this.store.findAll('namespace'),
      nodePools: this.store.findAll('node-pool'),
    });
  }

  startWatchers(controller, model) {
    controller.set('namespacesWatch', this.watchNamespaces.perform());
    // controller.set(
    //   'modelWatch',
    //   this.watchJobs.perform({ namespace: controller.qpNamespace, meta: true })
    // );
    controller.set(
      'jobsWatch',
      this.watchJobs.perform({
        namespace: controller.qpNamespace,
        meta: true,
        queryType: 'update',
        jobs: model.jobs.map((job) => {
          // TODO: maybe this should be set on controller for user-controlled updates?
          return {
            id: job.plainId,
            namespace: job.belongsTo('namespace').id(),
          };
        }),
      })
    );
  }

  @watchQuery('job') watchJobs;
  // @watchQuery('job', { queryType: 'update' }) watchJobsUpdate;
  @watchAll('namespace') watchNamespaces;
  @collect('watchJobs', 'watchNamespaces') watchers;
}
