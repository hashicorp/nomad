import { inject as service } from '@ember/service';
import { computed, get } from '@ember/object';
import RESTAdapter from 'ember-data/adapters/rest';
import codesForError from '../utils/codes-for-error';
import removeRecord from '../utils/remove-record';

export const namespace = 'v1';

export default RESTAdapter.extend({
  namespace,

  token: service(),

  headers: computed('token.secret', function() {
    const token = this.get('token.secret');
    if (token) {
      return {
        'X-Nomad-Token': token,
      };
    }
  }),

  findAll() {
    return this._super(...arguments).catch(error => {
      const errorCodes = codesForError(error);

      const isNotImplemented = errorCodes.includes('501');

      if (isNotImplemented) {
        return [];
      }

      // Rethrow to be handled downstream
      throw error;
    });
  },

  // In order to remove stale records from the store, findHasMany has to unload
  // all records related to the request in question.
  findHasMany(store, snapshot, link, relationship) {
    return this._super(...arguments).then(payload => {
      const relationshipType = relationship.type;
      const inverse = snapshot.record.inverseFor(relationship.key);
      if (inverse) {
        store
          .peekAll(relationshipType)
          .filter(record => record.get(`${inverse.name}.id`) === snapshot.id)
          .forEach(record => {
            removeRecord(store, record);
          });
      }
      return payload;
    });
  },

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
