import Route from '@ember/routing/route';

export default Route.extend({
  setupController(controller) {
    this._super(...arguments);
    controller.get('pollStats').perform();
  },

  resetController(controller) {
    controller.get('pollStats').cancelAll();
  },
});
