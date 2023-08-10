/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { inject as service } from '@ember/service';

export default class EvaluationsRoute extends Route.extend(WithWatchers) {
  @service store;

  model() {
    const job = this.modelFor('jobs.job');
    return job && job.get('evaluations').then(() => job);
  }

  startWatchers(controller, model) {
    if (model) {
      controller.set('watchEvaluations', this.watchEvaluations.perform(model));
    }
  }

  @watchRelationship('evaluations') watchEvaluations;

  @collect('watchEvaluations') watchers;
}
