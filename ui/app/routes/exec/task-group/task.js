import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

export default Route.extend({
  store: service(),

  model({ task_name }) {
    const allocationQueryParam = this.paramsFor('exec').allocation;

    return this.modelFor('exec').allocations.then(allocations => {
      let allocation;

      if (allocationQueryParam) {
        allocation = allocations.findBy('shortId', allocationQueryParam);
      } else {
        allocation = allocations.find(allocation =>
          allocation.states
            .filterBy('isActive')
            .mapBy('name')
            .includes(task_name)
        );
      }

      return {
        allocation,
        allocationSpecified: allocationQueryParam ? true : false,
        taskState: allocation.states.find(state => state.name === task_name),
      };
    });
  },

  afterModel(model) {
    this.controllerFor('exec').send('setTaskState', model);
  },

  setupController(controller, { allocation, taskState }) {
    controller.setProperties({ allocation, taskState });
  },
});
