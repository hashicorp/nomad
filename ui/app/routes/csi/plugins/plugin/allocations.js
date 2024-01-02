/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { inject as service } from '@ember/service';

export default class AllocationsRoute extends Route.extend(WithWatchers) {
  @service store;

  startWatchers(controller, model) {
    if (!model) return;

    controller.set('watchers', {
      model: this.watch.perform(model),
    });
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.setProperties({
        currentPage: 1,
        qpType: '',
        qpHealth: '',
      });
    }
  }

  @watchRecord('plugin') watch;
  @collect('watch') watchers;
}
