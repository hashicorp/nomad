import Ember from 'ember';
import RESTAdapter from 'ember-data/adapters/rest';

const { isArray, typeOf, get } = Ember;

export default RESTAdapter.extend({
  namespace: 'v1',

  // findAll() {
  //   return this._super(...arguments).then(data => {
  //     data.forEach(transformKeys);
  //     return data;
  //   });
  // },

  // findMany() {
  //   return this._super(...arguments).then(data => {
  //     data.forEach(transformKeys);
  //     return data;
  //   });
  // },

  // findRecord() {
  //   return this._super(...arguments).then(data => {
  //     transformKeys(data);
  //     return data;
  //   });
  // },

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
