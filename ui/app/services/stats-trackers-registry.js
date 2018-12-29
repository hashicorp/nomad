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

export default Service.extend({
  token: service(),

  init() {
    // The LRUMap limits the number of trackers tracked by making room for
    // new entries beyond the limit by removing the least recently used entry.
    registry = new LRUMap(MAX_STAT_TRACKERS);
  },

  // A read-only way of getting a reference to the registry.
  // Since this could be overwritten by a bad actor, it isn't
  // used in getTracker
  registryRef: computed(() => registry),

  getTracker(resource) {
    if (!resource) return;

    const type = resource && resource.constructor.modelName;
    const key = `${type}:${resource.get('id')}`;

    const cachedTracker = registry.get(key);
    if (cachedTracker) return cachedTracker;

    const Constructor = type === 'node' ? NodeStatsTracker : AllocationStatsTracker;
    const resourceProp = type === 'node' ? 'node' : 'allocation';

    const tracker = Constructor.create({
      fetch: url => this.get('token').authorizedRequest(url),
      [resourceProp]: resource,
    });

    registry.set(key, tracker);

    return tracker;
  },
});
