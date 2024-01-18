/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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
    console.log('jax1', url, type, options);
    const hasParams = hasNonBlockingQueryParams(options);
    console.log('ajax and hasParamas', hasParams, options);
    // TODO: TEMP. PROBLEM APPEARS TO BE THIS !GET RETURN
    // if (!hasParams || type !== 'GET') return super.ajax(url, type, options);
    if (!hasParams) return super.ajax(url, type, options);

    const params = { ...options.data };
    delete params.index;

    // Options data gets appended as query params as part of ajaxOptions.
    // In order to prevent doubling params, data should only include index
    // at this point since everything else is added to the URL in advance.
    // ^--- TODO: this is suitable for GETs but not POSTs!
    if (type === 'GET') {
      options.data = options.data.index ? { index: options.data.index } : {};
    }
    console.log('AJAXING', url, queryString.stringify(params));
    // TODO: TEMP HACK
    if (url === '/v1/jobs/statuses3' && type === 'POST') {
      const params = { index: options.data.index };
      console.log('options then', options, url, params);
      // Only keep the index param in params
      // options.data.index = 3528;
      // return super.ajax(`${url}?index=${options.data.index}`, type, options);
      return super.ajax(
        `${url}?${queryString.stringify(params)}`,
        type,
        options
      );
    } else {
      return super.ajax(
        `${url}?${queryString.stringify(params)}`,
        type,
        options
      );
    }
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
    console.log('watchlist', this.watchList);
    const builtURL = this.buildURL(type.modelName, null, null, 'query', query);
    const url = options.url || builtURL;
    console.log(
      'building URL from query in watchable adapter',
      options.url,
      builtURL
    );
    const method = get(options, 'adapterOptions.method') || 'GET';
    let [urlPath, params] = url.split('?');
    params = assign(
      queryString.parse(params) || {},
      this.buildQuery(),
      additionalParams,
      query
    );
    console.log('+++ params', params);
    if (get(options, 'adapterOptions.watch')) {
      // The intended query without additional blocking query params is used
      // to track the appropriate query index.
      // Is there a known index?
      console.log(
        'known index? +++',
        get(options, 'adapterOptions.knownIndex')
      );
      // TODO: TEMP HACKY
      // If there's an index in the watchlist for this url, use it.
      // Otherwise, check for knownIndex or possibly for the initialize one here
      // and use that.

      // TODO: queryString.stringify(query) has more than I need. I dont need it to include jobs for example.
      console.log('queryString', query, queryString.stringify(query));
      let watchListIndex = this.watchList.getIndexFor(
        `${urlPath}?${queryString.stringify(query)}`
      );
      console.log(
        'iii urlpath, watchListIndex',
        `${urlPath}?${queryString.stringify(query)}`,
        watchListIndex
      );
      if (watchListIndex > 1) {
        console.log('xxx if condition', watchListIndex);
        params.index = watchListIndex;
      } else if (get(options, 'adapterOptions.knownIndex')) {
        params.index = get(options, 'adapterOptions.knownIndex');
        console.log('xxx else condition', params.index);
      } else {
        params.index = 1;
      }
      // if (get(options, 'adapterOptions.knownIndex')) {
      //   console.log('so there should be already something at /v1/jobs/statuses3?meta=true&queryType=initialize');
      //   console.log(this.watchList.getIndexFor('/v1/jobs/statuses3?meta=true&queryType=initialize'));
      //   params.index = get(options, 'adapterOptions.knownIndex');
      //   // this.watchList.setIndexFor(
      //   //   `${urlPath}?${queryString.stringify(query)}`,
      //   //   get(options, 'adapterOptions.knownIndex')
      //   // );
      //   // params.index = this.watchList.getIndexFor(
      //   //   `${urlPath}?${queryString.stringify(query)}`
      //   // );
      //   // console.log('+++', params.index);
      //   params.jobs = [{ id: query.jobs[0].id }];
      // } else {
      //   params.index = this.watchList.getIndexFor(
      //     `${urlPath}?${queryString.stringify(query)}`
      //   );
      // }
      console.log(
        '+++ adapterOptions.watch is true, params.index is',
        params.index,
        params.jobs
      );
    }

    const signal = get(options, 'adapterOptions.abortController.signal');
    console.log('+++ SIGNAL SET AND PARAMS?', params);
    return this.ajax(urlPath, method, {
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

      // TODO: NOTE: MAYBE JUST DOESNT WORK ??
      // console.log('about to peek and eventually qPTA / removeRecord', {
      //   type,
      //   queryParamsToAttrs,
      //   query,
      // });
      store
        .peekAll(type.modelName)
        .filter((record) => {
          return queryParamsToAttrs.some(
            (mapping) => get(record, mapping.attr) === query[mapping.queryParam]
          );
        })
        .forEach((record) => {
          removeRecord(store, record);
        });

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
    // console.log('handling response with index of', newIndex);
    if (newIndex) {
      console.log(
        'iii response handled! new index!',
        requestData.url,
        newIndex
      );
      // console.log('+++new index, setting on watchlist', newIndex, requestData.url);
      this.watchList.setIndexFor(requestData.url, newIndex);
      // console.log('ok set new item on watchlist', this.watchList.list, this.watchList._list);
      // console.log("but what if I fetch?", this.watchList.getIndexFor(requestData.url));
      // debugger;
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
