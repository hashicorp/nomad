import Ember from 'ember';

const { Route } = Ember;

export default Route.extend({
  resetController(controller, isExiting) {
    if (isExiting) {
      controller.set('error', null);
    }
  },

  actions: {
    didTransition() {
      window.scrollTo(0, 0);
    },

    willTransition() {
      this.controllerFor('application').set('error', null);
    },

    error(error) {
      this.controllerFor('application').set('error', error);
    },
  },
});
