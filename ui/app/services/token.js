import Service, { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { getOwner } from '@ember/application';
import { assign } from '@ember/polyfills';
import { task } from 'ember-concurrency';
import queryString from 'query-string';
import fetch from 'nomad-ui/utils/fetch';
import classic from 'ember-classic-decorator';

@classic
export default class TokenService extends Service {
  @service store;
  @service system;

  aclEnabled = true;

  @computed
  get secret() {
    return window.localStorage.nomadTokenSecret;
  }

  set secret(value) {
    if (value == null) {
      window.localStorage.removeItem('nomadTokenSecret');
    } else {
      window.localStorage.nomadTokenSecret = value;
    }

    return value;
  }

  @task(function*() {
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
  })
  fetchSelfToken;

  @computed('secret', 'fetchSelfToken.lastSuccessful.value')
  get selfToken() {
    if (this.secret) return this.get('fetchSelfToken.lastSuccessful.value');
    return undefined;
  }

  @task(function*() {
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
  })
  fetchSelfTokenPolicies;

  @alias('fetchSelfTokenPolicies.lastSuccessful.value') selfTokenPolicies;

  @task(function*() {
    yield this.fetchSelfToken.perform();
    if (this.aclEnabled) {
      yield this.fetchSelfTokenPolicies.perform();
    }
  })
  fetchSelfTokenAndPolicies;

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
  }

  authorizedRequest(url, options) {
    if (this.get('system.shouldIncludeRegion')) {
      const region = this.get('system.activeRegion');
      if (region) {
        url = addParams(url, { region });
      }
    }

    return this.authorizedRawRequest(url, options);
  }

  reset() {
    this.fetchSelfToken.cancelAll({ resetState: true });
    this.fetchSelfTokenPolicies.cancelAll({ resetState: true });
    this.fetchSelfTokenAndPolicies.cancelAll({ resetState: true });
  }
}

function addParams(url, params) {
  const paramsStr = queryString.stringify(params);
  const delimiter = url.includes('?') ? '&' : '?';
  return `${url}${delimiter}${paramsStr}`;
}
