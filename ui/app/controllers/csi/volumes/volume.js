import Controller from '@ember/controller';

export default Controller.extend({
  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
