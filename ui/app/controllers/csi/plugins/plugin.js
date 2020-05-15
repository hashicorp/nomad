import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';

export default Controller.extend({
  controllerAllocation: alias('model.controllerAllocation'),
  nodeAllocation: alias('model.nodeAllocation'),

  plugin: alias('model.plugin'),

  sortedControllers: computed('plugin.controllers.@each.updateTime', function() {
    return this.plugin.controllers.sortBy('updateTime').reverse();
  }),

  sortedNodes: computed('plugin.nodes.@each.updateTime', function() {
    return this.plugin.nodes.sortBy('updateTime').reverse();
  }),

  actions: {
    gotoAllocation(allocation) {
      this.transitionToRoute('allocations.allocation', allocation);
    },
  },
});
