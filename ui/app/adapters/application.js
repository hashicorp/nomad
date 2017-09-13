import Ember from 'ember';
import RESTAdapter from 'ember-data/adapters/rest';

const { get, computed, inject } = Ember;

export const namespace = 'v1';

export default RESTAdapter.extend({
  namespace,

  token: inject.service(),

  headers: computed('token.secret', function() {
    const token = this.get('token.secret');
    return (
      token && {
        'X-Nomad-Token': token,
      }
    );
  }),

  // Single record requests deviate from REST practice by using
  // the singular form of the resource name.
  //
  // REST:  /some-resources/:id
  // Nomad: /some-resource/:id
  //
  // This is the original implementation of _buildURL
  // without the pluralization of modelName
  urlForFindRecord(id, modelName) {
    let path;
    let url = [];
    let host = get(this, 'host');
    let prefix = this.urlPrefix();

    if (modelName) {
      path = modelName.camelize();
      if (path) {
        url.push(path);
      }
    }

    if (id) {
      url.push(encodeURIComponent(id));
    }

    if (prefix) {
      url.unshift(prefix);
    }

    url = url.join('/');
    if (!host && url && url.charAt(0) !== '/') {
      url = '/' + url;
    }

    return url;
  },
});
