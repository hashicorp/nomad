/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import {
  watchRecord,
  watchRelationship,
  watchAll,
  watchQuery,
} from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default class IndexRoute extends Route.extend(WithWatchers) {
  @service can;
  @service store;

  async model() {
    return this.modelFor('jobs.job');
  }

  startWatchers(controller, model) {
    if (!model) {
      return;
    }
    controller.set('watchers', {
      summary: this.watchSummary.perform(model.get('summary')),
      allocations: this.watchAllocations.perform(model),
      evaluations: this.watchEvaluations.perform(model),
      latestDeployment:
        model.get('supportsDeployments') &&
        this.watchLatestDeployment.perform(model),
      list:
        model.get('hasChildren') &&
        this.watchAllJobs.perform({
          namespace: model.namespace.get('name'),
          meta: true,
        }),
      nodes:
        model.get('hasClientStatus') &&
        this.can.can('read client') &&
        this.watchNodes.perform(),
    });
  }

  setupController(controller, model) {
    // Parameterized and periodic detail pages, which list children jobs,
    // should sort by submit time.
    if (model && ['periodic', 'parameterized'].includes(model.templateType)) {
      controller.setProperties({
        sortProperty: 'submitTime',
        sortDescending: true,
      });
    }
    return super.setupController(...arguments);
  }

  @watchQuery('job') watchAllJobs;
  @watchAll('node') watchNodes;
  @watchRecord('job-summary') watchSummary;
  @watchRelationship('allocations') watchAllocations;
  @watchRelationship('evaluations') watchEvaluations;
  @watchRelationship('latestDeployment') watchLatestDeployment;

  @collect(
    'watchAllJobs',
    'watchSummary',
    'watchAllocations',
    'watchEvaluations',
    'watchLatestDeployment',
    'watchNodes'
  )
  watchers;
}
