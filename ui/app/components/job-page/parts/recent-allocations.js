import Component from '@ember/component';
import { computed } from '@ember/object';
import PromiseArray from 'nomad-ui/utils/classes/promise-array';

export default Component.extend({
  classNames: ['boxed-section'],

  sortProperty: 'modifyIndex',
  sortDescending: true,
  sortedAllocations: computed('job.allocations.@each.modifyIndex', function() {
    return new PromiseArray({
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
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
