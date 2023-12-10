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
} from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default class ClientRoute extends Route.extend(WithWatchers) {
  @service store;

  setupController(controller, model) {
    controller.set('flagAsDraining', model && model.isDraining);

    return super.setupController(...arguments);
  }

  resetController(controller) {
    controller.setProperties({
      eligibilityError: null,
      stopDrainError: null,
      drainError: null,
      flagAsDraining: false,
      showDrainNotification: false,
      showDrainUpdateNotification: false,
      showDrainStoppedNotification: false,
    });
  }

  startWatchers(controller, model) {
    if (model) {
      controller.set('watchModel', this.watch.perform(model));
      controller.set('watchAllocations', this.watchAllocations.perform(model));
    }
  }

  @watchRecord('node') watch;
  @watchRelationship('allocations') watchAllocations;

  @collect('watch', 'watchAllocations') watchers;
}
