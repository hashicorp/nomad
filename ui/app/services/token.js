import Service, { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { getOwner } from '@ember/application';
import { assign } from '@ember/polyfills';
import { task } from 'ember-concurrency';
import queryString from 'query-string';
import fetch from 'nomad-ui/utils/fetch';

export default Service.extend({
  store: service(),
  system: service(),

  aclEnabled: true,

  secret: computed({
    get() {
      return window.localStorage.nomadTokenSecret;
    },
    set(key, value) {
      if (value == null) {
        window.localStorage.removeItem('nomadTokenSecret');
      } else {
        window.localStorage.nomadTokenSecret = value;
      }

      return value;
    },
  }),

  fetchSelfToken: task(function*() {
    const TokenAdapter = getOwner(this).lookup('adapter:token');
    try {
      return yield TokenAdapter.findSelf();
    } catch (e) {
      const errors = e.errors ? e.errors.mapBy('detail') : [];
      if (errors.find(error => error === 'ACL support disabled')) {
        this.set('aclEnabled', false);
      }
      return null;
    }
  }),

  selfToken: computed('secret', 'fetchSelfToken.lastSuccessful.value', function() {
    if (this.secret) return this.get('fetchSelfToken.lastSuccessful.value');
  }),

  fetchSelfTokenPolicies: task(function*() {
    try {
      if (this.selfToken) {
        return yield this.selfToken.get('policies');
      } else {
        let policy = yield this.store.findRecord('policy', 'anonymous');
        return [policy];
      }
    } catch (e) {
      return [];
    }
  }),

  selfTokenPolicies: alias('fetchSelfTokenPolicies.lastSuccessful.value'),

  fetchSelfTokenAndPolicies: task(function*() {
    yield this.fetchSelfToken.perform();
    if (this.aclEnabled) {
      yield this.fetchSelfTokenPolicies.perform();
    }
  }),

  // All non Ember Data requests should go through authorizedRequest.
  // However, the request that gets regions falls into that category.
  // This authorizedRawRequest is necessary in order to fetch data
  // with the guarantee of a token but without the automatic region
  // param since the region cannot be known at this point.
  authorizedRawRequest(url, options = {}) {
    const credentials = 'include';
    const headers = {};
    const token = this.secret;

    if (token) {
      headers['X-Nomad-Token'] = token;
    }

    return fetch(url, assign(options, { headers, credentials }));
  },

  authorizedRequest(url, options) {
    if (this.get('system.shouldIncludeRegion')) {
      const region = this.get('system.activeRegion');
      if (region) {
        url = addParams(url, { region });
      }
    }

    return this.authorizedRawRequest(url, options);
  },

  reset() {
    this.fetchSelfToken.cancelAll({ resetState: true });
    this.fetchSelfTokenPolicies.cancelAll({ resetState: true });
    this.fetchSelfTokenAndPolicies.cancelAll({ resetState: true });
  },
});

function addParams(url, params) {
  const paramsStr = queryString.stringify(params);
  const delimiter = url.includes('?') ? '&' : '?';
  return `${url}${delimiter}${paramsStr}`;
}
