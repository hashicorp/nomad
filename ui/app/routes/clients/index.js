/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { service } from '@ember/service';

export default class IndexRoute extends Route.extend(WithWatchers) {
  @service store;

  startWatchers(controller) {
    controller.set('watcher', this.watch.perform());
    controller.set('watcherAllocations', this.watchAllocations.perform());
  }

  // Single shared long-poll for all nodes in the list.
  @watchAll('node') watch;

  // Single shared long-poll for all allocations.
  // Keeps node.runningAllocations up to date for every row without each row
  // opening its own /v1/node/:id/allocations blocking-query connection.
  @watchAll('allocation') watchAllocations;

  @collect('watch', 'watchAllocations') watchers;
}
