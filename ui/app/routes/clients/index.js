/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { inject as service } from '@ember/service';

export default class IndexRoute extends Route.extend(WithWatchers) {
  @service store;

  deactivate() {
    const startTime = performance.now();
    console.log('🔴 [CLIENTS] Route deactivating at', new Date().toISOString());

    this.cancelAllWatchers();

    super.deactivate(...arguments);

    const duration = performance.now() - startTime;
    console.log(
      `🔴 [CLIENTS] Deactivate completed in ${duration.toFixed(2)}ms`
    );
  }

  startWatchers(controller) {
    controller.set('watcher', this.watch.perform());
  }

  @watchAll('node') watch;
  @collect('watch') watchers;
}
