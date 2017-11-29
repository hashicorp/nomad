import Ember from 'ember';
import fetch from 'nomad-ui/utils/fetch';

const { Service, computed, assign } = Ember;

export default Service.extend({
  secret: computed({
    get() {
      return window.sessionStorage.nomadTokenSecret;
    },
    set(key, value) {
      if (value == null) {
        window.sessionStorage.removeItem('nomadTokenSecret');
      } else {
        window.sessionStorage.nomadTokenSecret = value;
      }

      return value;
    },
  }),

  authorizedRequest(url, options = {}) {
    const headers = {};
    const token = this.get('secret');

    if (token) {
      headers['X-Nomad-Token'] = token;
    }

    return fetch(url, assign(options, { headers }));
  },
});
