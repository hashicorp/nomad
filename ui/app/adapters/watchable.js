import { get, computed } from '@ember/object';
import { assign } from '@ember/polyfills';
import { inject as service } from '@ember/service';
import queryString from 'npm:query-string';
import ApplicationAdapter from './application';
import { AbortError } from 'ember-data/adapters/errors';

export default ApplicationAdapter.extend({
  watchList: service(),
  store: service(),

  xhrs: computed(function() {
    return {
      list: {},
      track(key, xhr) {
        if (this.list[key]) {
          this.list[key].push(xhr);
        } else {
          this.list[key] = [xhr];
        }
      },
      cancel(key) {
        while (this.list[key] && this.list[key].length) {
          this.remove(key, this.list[key][0]);
        }
      },
      remove(key, xhr) {
        if (this.list[key]) {
          xhr.abort();
          this.list[key].removeObject(xhr);
        }
      },
    };
  }),

  ajaxOptions() {
    const ajaxOptions = this._super(...arguments);
    const key = this.xhrKey(...arguments);

    const previousBeforeSend = ajaxOptions.beforeSend;
    ajaxOptions.beforeSend = function(jqXHR) {
      if (previousBeforeSend) {
        previousBeforeSend(...arguments);
      }
      this.get('xhrs').track(key, jqXHR);
      jqXHR.always(() => {
        this.get('xhrs').remove(key, jqXHR);
      });
    };

    return ajaxOptions;
  },

  xhrKey(url /* method, options */) {
    return url;
  },

  findAll(store, type, sinceToken, snapshotRecordArray, additionalParams = {}) {
    const params = assign(this.buildQuery(), additionalParams);
    const url = this.urlForFindAll(type.modelName);

    if (get(snapshotRecordArray || {}, 'adapterOptions.watch')) {
      params.index = this.get('watchList').getIndexFor(url);
    }

    return this.ajax(url, 'GET', {
      data: params,
    });
  },

  findRecord(store, type, id, snapshot, additionalParams = {}) {
    let [url, params] = this.buildURL(type.modelName, id, snapshot, 'findRecord').split('?');
    params = assign(queryString.parse(params) || {}, this.buildQuery(), additionalParams);

    if (get(snapshot || {}, 'adapterOptions.watch')) {
      params.index = this.get('watchList').getIndexFor(url);
    }

    return this.ajax(url, 'GET', {
      data: params,
    }).catch(error => {
      if (error instanceof AbortError) {
        return;
      }
      throw error;
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

      // Avoid duplicating existing query params by passing them to ajax
      // in the URL and in options.data
      if (url.includes('?')) {
        const paramsInUrl = queryString.parse(url.split('?')[1]);
        Object.keys(paramsInUrl).forEach(key => {
          delete params[key];
        });
      }

      return this.ajax(url, 'GET', {
        data: params,
      }).then(
        json => {
          const store = this.get('store');
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
      this.get('watchList').setIndexFor(requestData.url, newIndex);
    }

    return this._super(...arguments);
  },

  cancelFindRecord(modelName, id) {
    if (!modelName || id == null) {
      return;
    }
    const url = this.urlForFindRecord(id, modelName);
    this.get('xhrs').cancel(url);
  },

  cancelFindAll(modelName) {
    if (!modelName) {
      return;
    }
    let url = this.urlForFindAll(modelName);
    const params = queryString.stringify(this.buildQuery());
    if (params) {
      url = `${url}?${params}`;
    }
    this.get('xhrs').cancel(url);
  },

  cancelReloadRelationship(model, relationshipName) {
    if (!model || !relationshipName) {
      return;
    }
    const relationship = model.relationshipFor(relationshipName);
    if (relationship.kind !== 'belongsTo' && relationship.kind !== 'hasMany') {
      throw new Error(
        `${relationship.key} must be a belongsTo or hasMany, instead it was ${relationship.kind}`
      );
    } else {
      const url = model[relationship.kind](relationship.key).link();
      this.get('xhrs').cancel(url);
    }
  },
});
