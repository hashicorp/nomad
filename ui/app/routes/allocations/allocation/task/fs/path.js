import Route from '@ember/routing/route';

export default Route.extend({
  model({ path }) {
    return {
      path: decodeURIComponent(path),
      task: this.modelFor('allocations.allocation.task'),
    };
  },

  setupController(controller, { path, task }) {
    this._super(...arguments);
    controller.setProperties({ path, model: task });
  },
});
