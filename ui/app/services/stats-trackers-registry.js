/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import Service, { inject as service } from '@ember/service';
import { LRUMap } from 'lru_map';
import NodeStatsTracker from 'nomad-ui/utils/classes/node-stats-tracker';
import AllocationStatsTracker from 'nomad-ui/utils/classes/allocation-stats-tracker';

// An unbounded number of stat trackers is a great way to gobble up all the memory
// on a machine. This max number is unscientific, but aims to balance losing
// stat trackers a user is likely to return to with preventing gc from freeing
// memory occupied by stat trackers a user is likely to no longer care about
const MAX_STAT_TRACKERS = 10;
let registry;

const exists = (tracker, prop) =>
  tracker.get(prop) &&
  !tracker.get(prop).isDestroyed &&
  !tracker.get(prop).isDestroying;

export default class StatsTrackersRegistryService extends Service {
  @service token;

  constructor() {
    super(...arguments);

    // The LRUMap limits the number of trackers tracked by making room for
    // new entries beyond the limit by removing the least recently used entry.
    registry = new LRUMap(MAX_STAT_TRACKERS);
  }

  // A read-only way of getting a reference to the registry.
  // Since this could be overwritten by a bad actor, it isn't
  // used in getTracker
  @computed
  get registryRef() {
    return registry;
  }

  getTracker(resource) {
    if (!resource) return;

    const type = resource && resource.constructor.modelName;
    const key = `${type}:${resource.get('id')}`;
    const Constructor =
      type === 'node' ? NodeStatsTracker : AllocationStatsTracker;
    const resourceProp = type === 'node' ? 'node' : 'allocation';

    const cachedTracker = registry.get(key);
    if (cachedTracker) {
      // It's possible for the resource on a cachedTracker to have been
      // deleted. Rebind it if that's the case.
      if (!exists(cachedTracker, resourceProp))
        cachedTracker.set(resourceProp, resource);
      return cachedTracker;
    }

    const tracker = Constructor.create({
      fetch: (url) => this.token.authorizedRequest(url),
      [resourceProp]: resource,
    });

    registry.set(key, tracker);

    return tracker;
  }
}
