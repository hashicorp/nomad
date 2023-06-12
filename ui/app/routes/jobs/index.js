/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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

  model(params) {
    return RSVP.hash({
      jobs: this.store
        .query('job', { namespace: params.qpNamespace, meta: true })
        .catch(notifyForbidden(this)),
      allocations: this.store.query('allocation', {
        resources: false,
        task_states: false,
        namespace: '*',
      }),
      namespaces: this.store.findAll('namespace'),
    });
  }

  startWatchers(controller) {
    controller.set('namespacesWatch', this.watchNamespaces.perform());
    controller.set(
      'modelWatch',
      this.watchJobs.perform({ namespace: controller.qpNamespace, meta: true })
    );
    controller.set(
      'allocationsWatch',
      this.watchAllocations.perform({
        resources: false,
        task_states: false,
        namespace: '*',
      })
    );
  }

  @watchQuery('job') watchJobs;
  @watchQuery('allocation') watchAllocations;
  @watchAll('namespace') watchNamespaces;
  @collect('watchJobs', 'watchNamespaces', 'watchAllocations') watchers;
}
