import Service, { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { assign } from '@ember/polyfills';
import queryString from 'query-string';
import fetch from 'nomad-ui/utils/fetch';

export default Service.extend({
  system: service(),

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

  // All non Ember Data requests should go through authorizedRequest.
  // However, the request that gets regions falls into that category.
  // This authorizedRawRequest is necessary in order to fetch data
  // with the guarantee of a token but without the automatic region
  // param since the region cannot be known at this point.
  authorizedRawRequest(url, options = { credentials: 'include' }) {
    const headers = {};
    const token = this.get('secret');

    if (token) {
      headers['X-Nomad-Token'] = token;
    }

    return fetch(url, assign(options, { headers }));
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
});

function addParams(url, params) {
  const paramsStr = queryString.stringify(params);
  const delimiter = url.includes('?') ? '&' : '?';
  return `${url}${delimiter}${paramsStr}`;
}
