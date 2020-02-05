import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

export default Route.extend({
  store: service(),

  model({ task_name }) {
    const allocationQueryParam = this.paramsFor('exec').allocation;

    return this.modelFor('exec').allocations.then(allocations => {
      if (allocationQueryParam) {
        return {
          allocation: allocations.findBy('shortId', allocationQueryParam),
          allocationSpecified: true,
          task_name,
        };
      } else {
        return {
          allocation: allocations.objectAt(0),
          allocationSpecified: false,
          task_name,
        };
      }
    });
  },

  afterModel(model) {
    // FIXME model doesn’t have a task, just a task_name
    this.controllerFor('exec').send('setAllocationAndTask', model);
  },

  setupController(controller, { allocation }) {
    controller.setProperties({ allocation });
  },
});
