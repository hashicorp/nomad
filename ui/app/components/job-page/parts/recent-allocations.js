import Component from '@ember/component';
import { computed } from '@ember/object';
import { inject as service } from '@ember/service';
import PromiseArray from 'nomad-ui/utils/classes/promise-array';

export default Component.extend({
  classNames: ['boxed-section'],

  router: service(),

  sortProperty: 'modifyIndex',
  sortDescending: true,
  sortedAllocations: computed('job.allocations.@each.modifyIndex', function() {
    return PromiseArray.create({
      promise: this.get('job.allocations').then(allocations =>
        allocations
          .sortBy('modifyIndex')
          .reverse()
          .slice(0, 5)
      ),
    });
  }),

  actions: {
    gotoAllocation(allocation) {
      this.router.transitionTo('allocations.allocation', allocation.id);
    },
  },
});
