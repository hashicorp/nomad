import Ember from 'ember';

const { Route, inject } = Ember;

export default Route.extend({
  config: inject.service(),

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.set('error', null);
    }
  },

  actions: {
    didTransition() {
      if (!this.get('config.isTest')) {
        window.scrollTo(0, 0);
      }
    },

    willTransition() {
      this.controllerFor('application').set('error', null);
    },

    error(error) {
      this.controllerFor('application').set('error', error);
    },
  },
});
