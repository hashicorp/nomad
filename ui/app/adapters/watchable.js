import { get } from '@ember/object';
import { assign } from '@ember/polyfills';
import { inject as service } from '@ember/service';
import queryString from 'query-string';
import ApplicationAdapter from './application';
import { AbortError } from 'ember-data/adapters/errors';

export default ApplicationAdapter.extend({
  watchList: service(),
  store: service(),

  ajaxOptions(url, type, options) {
    const ajaxOptions = this._super(...arguments);
    const abortToken = (options || {}).abortToken;
    if (abortToken) {
      delete options.abortToken;

      const previousBeforeSend = ajaxOptions.beforeSend;
      ajaxOptions.beforeSend = function(jqXHR) {
        abortToken.capture(jqXHR);
        if (previousBeforeSend) {
          previousBeforeSend(...arguments);
        }
      };
    }

    return ajaxOptions;
  },

  findAll(store, type, sinceToken, snapshotRecordArray, additionalParams = {}) {
    const params = assign(this.buildQuery(), additionalParams);
    const url = this.urlForFindAll(type.modelName);

    if (get(snapshotRecordArray || {}, 'adapterOptions.watch')) {
      params.index = this.watchList.getIndexFor(url);
    }

    const abortToken = get(snapshotRecordArray || {}, 'adapterOptions.abortToken');
    return this.ajax(url, 'GET', {
      abortToken,
      data: params,
    });
  },

  findRecord(store, type, id, snapshot, additionalParams = {}) {
    let [url, params] = this.buildURL(type.modelName, id, snapshot, 'findRecord').split('?');
    params = assign(queryString.parse(params) || {}, this.buildQuery(), additionalParams);

    if (get(snapshot || {}, 'adapterOptions.watch')) {
      params.index = this.watchList.getIndexFor(url);
    }

    const abortToken = get(snapshot || {}, 'adapterOptions.abortToken');
    return this.ajax(url, 'GET', {
      abortToken,
      data: params,
    }).catch(error => {
      if (error instanceof AbortError) {
        return;
      }
      throw error;
    });
  },

  reloadRelationship(model, relationshipName, options = { watch: false, abortToken: null }) {
    const { watch, abortToken } = options;
    const relationship = model.relationshipFor(relationshipName);
    if (relationship.kind !== 'belongsTo' && relationship.kind !== 'hasMany') {
      throw new Error(
        `${relationship.key} must be a belongsTo or hasMany, instead it was ${relationship.kind}`
      );
    } else {
      const url = model[relationship.kind](relationship.key).link();
      let params = {};

      if (watch) {
        params.index = this.watchList.getIndexFor(url);
      }

      // Avoid duplicating existing query params by passing them to ajax
      // in the URL and in options.data
      if (url.includes('?')) {
        const paramsInUrl = queryString.parse(url.split('?')[1]);
        Object.keys(paramsInUrl).forEach(key => {
          delete params[key];
        });
      }

      return this.ajax(url, 'GET', {
        abortToken,
        data: params,
      }).then(
        json => {
          const store = this.store;
          const normalizeMethod =
            relationship.kind === 'belongsTo'
              ? 'normalizeFindBelongsToResponse'
              : 'normalizeFindHasManyResponse';
          const serializer = store.serializerFor(relationship.type);
          const modelClass = store.modelFor(relationship.type);
          const normalizedData = serializer[normalizeMethod](store, modelClass, json);
          store.push(normalizedData);
        },
        error => {
          if (error instanceof AbortError) {
            return relationship.kind === 'belongsTo' ? {} : [];
          }
          throw error;
        }
      );
    }
  },

  handleResponse(status, headers, payload, requestData) {
    // Some browsers lowercase all headers. Others keep them
    // case sensitive.
    const newIndex = headers['x-nomad-index'] || headers['X-Nomad-Index'];
    if (newIndex) {
      this.watchList.setIndexFor(requestData.url, newIndex);
    }

    return this._super(...arguments);
  },
});
