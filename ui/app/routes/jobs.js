import Ember from 'ember';

const { Route, inject, run } = Ember;

export default Route.extend({
  system: inject.service(),
  store: inject.service(),

  beforeModel() {
    return this.get('system.namespaces');
  },

  model() {
    return this.get('store').findAll('job');
  },

  syncToController(controller) {
    const namespace = this.get('system.activeNamespace.id');

    // The run next is necessary to let the controller figure
    // itself out before updating QPs.
    // See: https://github.com/emberjs/ember.js/issues/5465
    run.next(() => {
      if (namespace && namespace !== 'default') {
        controller.set('jobNamespace', namespace);
      } else {
        controller.set('jobNamespace', 'default');
      }
    });
  },

  setupController(controller) {
    this.syncToController(controller);
    return this._super(...arguments);
  },

  actions: {
    refreshRoute() {
      this.refresh();
    },
  },
});
