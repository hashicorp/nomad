import Ember from 'ember';

const { Route, inject } = Ember;

export default Route.extend({
  system: inject.service(),
  store: inject.service(),

  beforeModel() {
    return this.get('system.namespaces');
  },

  model() {
    return this.get('store').findAll('job');
  },

  actions: {
    refreshRoute() {
      this.refresh();
    },
  },
});
