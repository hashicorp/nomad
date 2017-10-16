import Ember from 'ember';

const { Route, inject } = Ember;

export default Route.extend({
  system: inject.service(),

  setupController(controller) {
    this._super(...arguments);

    const namespace = this.get('system.activeNamespace.id');
    if (namespace && namespace !== 'default') {
      controller.set('jobNamespace', namespace);
    } else {
      controller.set('jobNamespace', 'default');
    }
  },
});
