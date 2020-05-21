import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  sortedControllers: computed('model.controllers.@each.updateTime', function() {
    return this.model.controllers.sortBy('updateTime').reverse();
  }),

  sortedNodes: computed('model.nodes.@each.updateTime', function() {
    return this.model.nodes.sortBy('updateTime').reverse();
  }),

  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
