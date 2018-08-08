import { inject as service } from '@ember/service';
import { next } from '@ember/runloop';
import Route from '@ember/routing/route';
import { AbortError } from 'ember-data/adapters/errors';
import RSVP from 'rsvp';

export default Route.extend({
  config: service(),
  system: service(),

  queryParams: {
    region: {
      refreshModel: true,
    },
  },

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.set('error', null);
    }
  },

  afterSetup(fn) {
    this._afterSetups || (this._afterSetups = []);
    this._afterSetups.push(fn);
  },

  beforeModel(transition) {
    return RSVP.all([this.get('system.regions'), this.get('system.namespaces')]).then(promises => {
      const queryParam = transition.queryParams.region;
      const activeRegion = this.get('system.activeRegion');

      if (!queryParam && activeRegion) {
        this.afterSetup(controller => {
          controller.set('region', activeRegion);
        });
      } else if (queryParam && queryParam !== activeRegion) {
        this.set('system.activeRegion', queryParam);
      }

      return promises;
    });
  },

  setupController(controller) {
    next(() => {
      (this._afterSetups || []).forEach(fn => {
        fn(controller);
      });
      this._afterSetups = [];
    });
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
