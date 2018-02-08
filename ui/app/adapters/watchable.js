import { get } from '@ember/object';
import { assign } from '@ember/polyfills';
import { copy } from '@ember/object/internals';
import { inject as service } from '@ember/service';
import queryString from 'npm:query-string';
import ApplicationAdapter from './application';

export default ApplicationAdapter.extend({
  watchList: service(),
  store: service(),

  findRecord(store, type, id, snapshot, additionalParams = {}) {
    const params = copy(additionalParams, true);
    const url = this.buildURL(type.modelName, id, snapshot, 'findRecord');

    if (get(snapshot, 'adapterOptions.watch')) {
      params.index = this.get('watchList').getIndexFor(url);
    }

    return this.ajax(url, 'GET', {
      data: params,
    });
  },

  reloadRelationship(model, relationshipName, watch = false) {
    const relationship = model.relationshipFor(relationshipName);
    if (relationship.kind !== 'belongsTo' && relationship.kind !== 'hasMany') {
      throw new Error(
        `${relationship.key} must be a belongsTo or hasMany, instead it was ${relationship.kind}`
      );
    } else {
      const url = model[relationship.kind](relationship.key).link();
      let params = {};

      if (watch) {
        params.index = this.get('watchList').getIndexFor(url);
      }

      if (url.includes('?')) {
        params = assign(queryString.parse(url.split('?')[1]), params);
      }

      this.ajax(url, 'GET', {
        data: params,
      }).then(json => {
        this.get('store').pushPayload(relationship.type, {
          [relationship.type]: relationship.kind === 'hasMany' ? json : [json],
        });
      });
    }
  },

  handleResponse(status, headers, payload, requestData) {
    const newIndex = headers['x-nomad-index'];
    if (newIndex) {
      this.get('watchList').setIndexFor(requestData.url, newIndex);
    }
    return this._super(...arguments);
  },
});
