import { get } from '@ember/object';
import { assign } from '@ember/polyfills';
import { inject as service } from '@ember/service';
import { AbortError } from '@ember-data/adapter/error';
import queryString from 'query-string';
import ApplicationAdapter from './application';
import removeRecord from '../utils/remove-record';
import classic from 'ember-classic-decorator';

@classic
export default class Watchable extends ApplicationAdapter {
  @service watchList;
  @service store;

  // Overriding ajax is not advised, but this is a minimal modification
  // that sets off a series of events that results in query params being
  // available in handleResponse below. Unfortunately, this is the only
  // place where what becomes requestData can be modified.
  //
  // It's either this weird side-effecting thing that also requires a change
  // to ajaxOptions or overriding ajax completely.
  ajax(url, type, options) {
    const hasParams = hasNonBlockingQueryParams(options);
    if (!hasParams || type !== 'GET') return super.ajax(url, type, options);

    const params = { ...options.data };
    delete params.index;

    // Options data gets appended as query params as part of ajaxOptions.
    // In order to prevent doubling params, data should only include index
    // at this point since everything else is added to the URL in advance.
    options.data = options.data.index ? { index: options.data.index } : {};

    return super.ajax(`${url}?${queryString.stringify(params)}`, type, options);
  }

  findAll(store, type, sinceToken, snapshotRecordArray, additionalParams = {}) {
    const params = assign(this.buildQuery(), additionalParams);
    const url = this.urlForFindAll(type.modelName);

    if (get(snapshotRecordArray || {}, 'adapterOptions.watch')) {
      params.index = this.watchList.getIndexFor(url);
    }

    const signal = get(
      snapshotRecordArray || {},
      'adapterOptions.abortController.signal'
    );
    return this.ajax(url, 'GET', {
      signal,
      data: params,
    });
  }

  findRecord(store, type, id, snapshot, additionalParams = {}) {
    const originalUrl = this.buildURL(
      type.modelName,
      id,
      snapshot,
      'findRecord'
    );
    let [url, params] = originalUrl.split('?');
    params = assign(
      queryString.parse(params) || {},
      this.buildQuery(),
      additionalParams
    );

    if (get(snapshot || {}, 'adapterOptions.watch')) {
      params.index = this.watchList.getIndexFor(originalUrl);
    }

    const signal = get(snapshot || {}, 'adapterOptions.abortController.signal');
    return this.ajax(url, 'GET', {
      signal,
      data: params,
    }).catch((error) => {
      if (error instanceof AbortError || error.name == 'AbortError') {
        return;
      }
      throw error;
    });
  }

  query(
    store,
    type,
    query,
    snapshotRecordArray,
    options,
    additionalParams = {}
  ) {
    const url = this.buildURL(type.modelName, null, null, 'query', query);
    let [urlPath, params] = url.split('?');
    params = assign(
      queryString.parse(params) || {},
      this.buildQuery(),
      additionalParams,
      query
    );
    // If your query is effectively a findAll under the hood, treat non-returned entities as deleted
    const isPseudoFindAll = Object.values(params).every((v) => v === '*');

    if (get(options, 'adapterOptions.watch')) {
      // The intended query without additional blocking query params is used
      // to track the appropriate query index.
      params.index = this.watchList.getIndexFor(
        `${urlPath}?${queryString.stringify(query)}`
      );
    }

    const signal = get(options, 'adapterOptions.abortController.signal');
    return this.ajax(urlPath, 'GET', {
      signal,
      data: params,
    }).then((payload) => {
      const adapter = store.adapterFor(type.modelName);

      // Query params may not necessarily map one-to-one to attribute names.
      // Adapters are responsible for declaring param mappings.
      const queryParamsToAttrs = Object.keys(
        adapter.queryParamsToAttrs || {}
      ).map((key) => ({
        queryParam: key,
        attr: adapter.queryParamsToAttrs[key],
      }));

      // Remove existing records that match this query. This way if server-side
      // deletes have occurred, the store won't have stale records.
      console.log(
        'watcher.query has been thenned; about to make a peekAll and filter out'
      );
      const matchingStoreEntries = store
        .peekAll(type.modelName)
        .filter((record) => {
          console.log('reco', queryParamsToAttrs, record);
          return queryParamsToAttrs.some((mapping) => {
            // Special consideration for * queries, like "All jobs with * namespace":
            // entities will generally have "default" instead of "*" as their value for these sorts of properties.
            const queryValue =
              query[mapping.queryParam] === '*'
                ? 'default'
                : query[mapping.queryParam];
            console.log(
              'SOME?',
              mapping.attr,
              query[mapping.queryParam],
              queryValue,
              get(record, mapping.attr),
              get(get(record, mapping.attr), 'id')
            );
            return (
              get(record, mapping.attr) === queryValue ||
              get(get(record, mapping.attr), 'id') === queryValue
            );
          });
        });
      console.log('=========', matchingStoreEntries);

      if (isPseudoFindAll) {
        matchingStoreEntries.forEach((record) => {
          const IDValue = record.get('plainId') || record.get('id');
          const storedRecordNotFoundInPayload =
            IDValue && !payload.find((r) => r.ID === IDValue);
          console.log(
            'storedRecordNotFoundInPayload for',
            record.id,
            payload.mapBy('ID'),
            storedRecordNotFoundInPayload
          );
          if (storedRecordNotFoundInPayload) {
            removeRecord(store, record);
          }
        });
      }

      console.log('type', type.modelName, payload);

      // payload.forEach((record) => {
      //   store.pushPayload(type.modelName, {
      //     [`${type.modelName}s`]: record
      //   });
      // });
      console.log(
        'payload',
        store.peekAll(type.modelName),
        matchingStoreEntries
      );

      return payload;
    });
  }

  reloadRelationship(
    model,
    relationshipName,
    options = { watch: false, abortController: null, replace: false }
  ) {
    const { watch, abortController, replace } = options;
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
        Object.keys(paramsInUrl).forEach((key) => {
          delete params[key];
        });
      }

      return this.ajax(url, 'GET', {
        signal: abortController && abortController.signal,
        data: params,
      }).then(
        (json) => {
          const store = this.store;
          const normalizeMethod =
            relationship.kind === 'belongsTo'
              ? 'normalizeFindBelongsToResponse'
              : 'normalizeFindHasManyResponse';
          const serializer = store.serializerFor(relationship.type);
          const modelClass = store.modelFor(relationship.type);
          const normalizedData = serializer[normalizeMethod](
            store,
            modelClass,
            json
          );
          if (replace) {
            store.unloadAll(relationship.type);
          }
          store.push(normalizedData);
        },
        (error) => {
          if (error instanceof AbortError || error.name === 'AbortError') {
            return relationship.kind === 'belongsTo' ? {} : [];
          }
          throw error;
        }
      );
    }
  }

  handleResponse(status, headers, payload, requestData) {
    // Some browsers lowercase all headers. Others keep them
    // case sensitive.
    const newIndex = headers['x-nomad-index'] || headers['X-Nomad-Index'];
    if (newIndex) {
      this.watchList.setIndexFor(requestData.url, newIndex);
    }

    return super.handleResponse(...arguments);
  }
}

function hasNonBlockingQueryParams(options) {
  if (!options || !options.data) return false;
  const keys = Object.keys(options.data);
  if (!keys.length) return false;
  if (keys.length === 1 && keys[0] === 'index') return false;

  return true;
}
