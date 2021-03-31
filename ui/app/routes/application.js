import { inject as service } from '@ember/service';
import { next } from '@ember/runloop';
import Route from '@ember/routing/route';
import { AbortError } from '@ember-data/adapter/error';
import RSVP from 'rsvp';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class ApplicationRoute extends Route {
  @service config;
  @service system;
  @service store;
  @service token;

  queryParams = {
    region: {
      refreshModel: true,
    },
  };

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.set('error', null);
    }
  }

  async beforeModel(transition) {
    let exchangeOneTimeToken;

    if (transition.to.queryParams.ott) {
      exchangeOneTimeToken = this.get('token').exchangeOneTimeToken(transition.to.queryParams.ott);
    } else {
      exchangeOneTimeToken = Promise.resolve(true);
    }

    try {
      await exchangeOneTimeToken;
    } catch (e) {
      this.controllerFor('application').set('error', e);
    }

    const fetchSelfTokenAndPolicies = this.get('token.fetchSelfTokenAndPolicies')
      .perform()
      .catch();

    const fetchLicense = this.get('system.fetchLicense')
      .perform()
      .catch();

    const promises = await RSVP.all([
      this.get('system.regions'),
      this.get('system.defaultRegion'),
      fetchLicense,
      fetchSelfTokenAndPolicies,
    ]);

    if (!this.get('system.shouldShowRegions')) return promises;

    const queryParam = transition.to.queryParams.region;
    const defaultRegion = this.get('system.defaultRegion.region');
    const currentRegion = this.get('system.activeRegion') || defaultRegion;

    // Only reset the store if the region actually changed
    if (
      (queryParam && queryParam !== currentRegion) ||
      (!queryParam && currentRegion !== defaultRegion)
    ) {
      this.system.reset();
      this.store.unloadAll();
    }

    this.set('system.activeRegion', queryParam || defaultRegion);

    return promises;
  }

  // Model is being used as a way to transfer the provided region
  // query param to update the controller state.
  model(params) {
    return params.region;
  }

  setupController(controller, model) {
    const queryParam = model;

    if (queryParam === this.get('system.defaultRegion.region')) {
      next(() => {
        controller.set('region', null);
      });
    }

    return super.setupController(...arguments);
  }

  @action
  didTransition() {
    if (!this.get('config.isTest')) {
      window.scrollTo(0, 0);
    }
  }

  @action
  willTransition() {
    this.controllerFor('application').set('error', null);
  }

  @action
  error(error) {
    if (!(error instanceof AbortError)) {
      this.controllerFor('application').set('error', error);
    }
  }
}
