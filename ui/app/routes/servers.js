import Ember from 'ember';

const { Route, inject, RSVP } = Ember;

export default Route.extend({
  store: inject.service(),
  system: inject.service(),

  beforeModel() {
    return this.get('system.leader');
  },

  model() {
    return RSVP.hash({
      nodes: this.get('store').findAll('node'),
      agents: this.get('store').findAll('agent'),
    });
  },
});
