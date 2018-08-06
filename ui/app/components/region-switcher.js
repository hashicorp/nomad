import Component from '@ember/component';
import { next } from '@ember/runloop';
import { computed } from '@ember/object';
import { inject as service } from '@ember/service';

export default Component.extend({
  system: service(),
  router: service(),
  store: service(),

  sortedRegions: computed('system.regions', function() {
    return this.get('system.regions')
      .toArray()
      .sort();
  }),

  gotoRegion(region) {
    this.get('system').reset();
    this.get('store').unloadAll();
    next(() => {
      this.get('router').transitionTo('jobs', {
        queryParams: { region },
      });
    });
  },
});
