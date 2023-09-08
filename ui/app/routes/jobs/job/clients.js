/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import {
  watchRecord,
  watchRelationship,
  watchAll,
} from 'nomad-ui/utils/properties/watch';
import { collect } from '@ember/object/computed';

export default class ClientsRoute extends Route.extend(WithWatchers) {
  @service can;
  @service store;

  beforeModel() {
    if (this.can.cannot('read client')) {
      this.transitionTo('jobs.job');
    }
  }

  async model() {
    return this.modelFor('jobs.job');
  }

  startWatchers(controller, model) {
    if (!model) {
      return;
    }

    controller.set('watchers', {
      model: this.watch.perform(model),
      allocations: this.watchAllocations.perform(model),
      nodes: this.watchNodes.perform(),
    });
  }

  @watchRecord('job') watch;
  @watchAll('node') watchNodes;
  @watchRelationship('allocations') watchAllocations;

  @collect('watch', 'watchNodes', 'watchAllocations')
  watchers;
}
