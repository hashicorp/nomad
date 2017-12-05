import Ember from 'ember';

const { Route } = Ember;

export default Route.extend({
  model() {
    const task = this._super(...arguments);
    return task.get('allocation.node').then(() => task);
  },
});
