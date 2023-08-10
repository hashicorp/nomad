/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { inject as service } from '@ember/service';

export default class IndexRoute extends Route.extend(WithWatchers) {
  @service store;

  startWatchers(controller) {
    controller.set('watcher', this.watch.perform());
  }

  @watchAll('node') watch;
  @collect('watch') watchers;
}
