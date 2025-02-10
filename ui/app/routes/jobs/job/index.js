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
} from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { action } from '@ember/object';

export default class IndexRoute extends Route.extend(WithWatchers) {
  @service can;
  @service store;
  @service watchList;

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

      controller.resetQueryIndex({
        id: model.get('plainId'),
        namespace: model.get('namespace.id'),
      });

      controller.watchChildJobs.perform({
        id: model.get('plainId'),
        namespace: model.get('namespace.id'),
      });
    }
    return super.setupController(...arguments);
  }

  @watchAll('node') watchNodes;
  @watchRecord('job-summary') watchSummary;
  @watchRelationship('allocations') watchAllocations;
  @watchRelationship('evaluations') watchEvaluations;
  @watchRelationship('latestDeployment') watchLatestDeployment;
  @collect(
    'watchSummary',
    'watchAllocations',
    'watchEvaluations',
    'watchLatestDeployment',
    'watchNodes'
  )
  watchers;

  @action
  willTransition() {
    // eslint-disable-next-line
    this.controller.childJobsController.abort();
    // eslint-disable-next-line
    this.controller.watchChildJobs.cancelAll();
    this.cancelAllWatchers();
    return true;
  }
}
