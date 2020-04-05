import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';

export default Controller.extend({
  system: service(),

  sortedReadAllocations: computed('model.readAllocations.@each.modifyIndex', function() {
    return this.model.readAllocations.sortBy('modifyIndex').reverse();
  }),

  sortedWriteAllocations: computed('model.writeAllocations.@each.modifyIndex', function() {
    return this.model.writeAllocations.sortBy('modifyIndex').reverse();
  }),

  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
