import Route from '@ember/routing/route';

export default Route.extend({
  model() {
    const task = this._super(...arguments);
    return task && task.get('allocation.node').then(() => task);
  },
});
