import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import { AbortError } from 'ember-data/adapters/errors';

export default Route.extend({
  config: service(),

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
      if (!(error instanceof AbortError)) {
        this.controllerFor('application').set('error', error);
      }
    },
  },
});
