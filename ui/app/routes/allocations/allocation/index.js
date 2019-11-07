import Route from '@ember/routing/route';

export default Route.extend({
  setupController(controller, model) {
    // Suppress the preemptedByAllocation fetch error in the event it's a 404
    if (model) {
      const setPreempter = () => controller.set('preempter', model.preemptedByAllocation);
      model.preemptedByAllocation.then(setPreempter, setPreempter);
    }

    return this._super(...arguments);
  },

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.watchNext.cancelAll();
    }
  },
});
