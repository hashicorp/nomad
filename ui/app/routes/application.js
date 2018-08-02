import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import { next } from '@ember/runloop';
import { AbortError } from 'ember-data/adapters/errors';

export default Route.extend({
  config: service(),
  system: service(),

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.set('error', null);
    }
  },

  beforeModel() {
    return this.get('system.regions');
  },

  syncToController(controller) {
    const region = this.get('system.activeRegion');

    // The run next is necessary to let the controller figure
    // itself out before updating QPs.
    // See: https://github.com/emberjs/ember.js/issues/5465
    next(() => {
      controller.set('region', region || 'global');
    });
  },

  setupController(controller) {
    this.syncToController(controller);
    return this._super(...arguments);
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
