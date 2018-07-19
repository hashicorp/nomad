import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  sortProperty: 'modifyIndex',
  sortDescending: true,
  sortedAllocations: computed('job.allocations.@each.modifyIndex', function() {
    return this.get('job.allocations')
      .sortBy('modifyIndex')
      .reverse()
      .slice(0, 5);
  }),

  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
