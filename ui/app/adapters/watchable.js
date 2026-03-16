/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';
import { macroCondition, isTesting } from '@embroider/macros';
import { AbortError } from '@ember-data/adapter/error';
import queryString from 'query-string';
import ApplicationAdapter from './application';
import removeRecord from '../utils/remove-record';
import classic from 'ember-classic-decorator';

const SHOULD_PRE_ADVANCE_WATCH_INDEX = macroCondition(isTesting())
  ? true
  : false;

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
  ajax(url, type, options = {}) {
    const hasParams = hasNonBlockingQueryParams(options);
    if (!hasParams || type !== 'GET') return super.ajax(url, type, options);
    let params = { ...options?.data };
    delete params.index;

    // Options data gets appended as query params as part of ajaxOptions.
    // In order to prevent doubling params, data should only include index
    // at this point since everything else is added to the URL in advance.
    options.data = options.data.index ? { index: options.data.index } : {};

    return super.ajax(`${url}?${queryString.stringify(params)}`, type, options);
  }

  findAll(store, type, sinceToken, snapshotRecordArray, additionalParams = {}) {
    const params = Object.assign(this.buildQuery(), additionalParams);
    const url = this.urlForFindAll(type.modelName);

    if (get(snapshotRecordArray || {}, 'adapterOptions.watch')) {
      const currentIndex = this.watchList.getIndexFor(url);
      params.index = currentIndex;
      if (shouldPreAdvanceWatchIndex()) {
        this.watchList.setIndexFor(url, nextWatchIndex(currentIndex));
      }
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
    params = Object.assign(
      queryString.parse(params) || {},
      this.buildQuery(),
      additionalParams
    );

    if (get(snapshot || {}, 'adapterOptions.watch')) {
      const currentIndex = this.watchList.getIndexFor(originalUrl);
      params.index = currentIndex;
      if (shouldPreAdvanceWatchIndex()) {
        this.watchList.setIndexFor(originalUrl, nextWatchIndex(currentIndex));
      }
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
    const method = get(options, 'adapterOptions.method') || 'GET';
    let [urlPath, params] = url.split('?');
    params = Object.assign(
      queryString.parse(params) || {},
      this.buildQuery(),
      additionalParams,
      query
    );

    if (get(options, 'adapterOptions.watch')) {
      const watchKey = `${urlPath}?${queryString.stringify(query)}`;
      const currentIndex = this.watchList.getIndexFor(watchKey);
      params.index = currentIndex;
      if (shouldPreAdvanceWatchIndex()) {
        this.watchList.setIndexFor(watchKey, nextWatchIndex(currentIndex));
      }
    }

    const signal = get(options, 'adapterOptions.abortController.signal');
    return this.ajax(urlPath, method, {
      signal,
      data: params,
    }).then((payload) => {
      if (!store || store.isDestroying || store.isDestroyed) {
        return payload;
      }

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
      store
        .peekAll(type.modelName)
        .filter((record) =>
          queryParamsToAttrs.some(
            (mapping) => get(record, mapping.attr) === query[mapping.queryParam]
          )
        )
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
    const store = lookupStore(this);
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
        const currentIndex = this.watchList.getIndexFor(url);
        params.index = currentIndex;
        if (shouldPreAdvanceWatchIndex()) {
          this.watchList.setIndexFor(url, nextWatchIndex(currentIndex));
        }
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
          if (!store || store.isDestroying || store.isDestroyed) {
            return json;
          }

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
    const headerIndex = getHeaderValue(headers, 'x-nomad-index');
    const fallbackIndex = shouldPreAdvanceWatchIndex()
      ? getNextIndexFromRequest(requestData)
      : null;
    const newIndex = headerIndex || fallbackIndex;

    if (
      newIndex &&
      hasWatchIndex(requestData) &&
      !this.isDestroying &&
      !this.isDestroyed
    ) {
      const watchList = lookupWatchList(this);

      if (watchList) {
        watchKeysForRequest(requestData).forEach((key) => {
          watchList.setIndexFor(key, newIndex);
        });
      }
    }

    return super.handleResponse(...arguments);
  }
}

function getHeaderValue(headers, name) {
  if (!headers) {
    return null;
  }

  if (typeof headers === 'string') {
    const target = name.toLowerCase();
    const match = headers
      .split(/\r?\n/)
      .map((line) => line.trim())
      .find((line) => line.toLowerCase().startsWith(`${target}:`));

    if (!match) {
      return null;
    }

    const separator = match.indexOf(':');
    return separator > -1 ? match.slice(separator + 1).trim() : null;
  }

  if (typeof headers.get === 'function') {
    return headers.get(name) || headers.get(name.toLowerCase());
  }

  return (
    headers[name] || headers[name.toLowerCase()] || headers[name.toUpperCase()]
  );
}

function normalizeWatchURL(url = '') {
  let path = url;
  let rawQuery = '';

  try {
    const parsed = new URL(url, window.location.origin);
    path = parsed.pathname;
    rawQuery = parsed.search.startsWith('?')
      ? parsed.search.slice(1)
      : parsed.search;
  } catch {
    [path, rawQuery = ''] = url.split('?');
  }

  if (!rawQuery) {
    return path;
  }

  const params = queryString.parse(rawQuery);
  delete params.index;

  const normalizedQuery = queryString.stringify(params);
  return normalizedQuery ? `${path}?${normalizedQuery}` : path;
}

function watchKeysForRequest(requestData = {}) {
  const keys = new Set();
  const normalizedUrl = normalizeWatchURL(requestData.url || '');

  if (normalizedUrl) {
    keys.add(normalizedUrl);
  }

  if (requestData.data && typeof requestData.data === 'object') {
    const params = { ...requestData.data };
    delete params.index;

    if (Object.keys(params).length) {
      const [path] = normalizedUrl.split('?');
      keys.add(`${path}?${queryString.stringify(params)}`);
    }
  }

  return [...keys];
}

function hasWatchIndex(requestData = {}) {
  const { url = '', data } = requestData;

  if (data && typeof data === 'object' && data.index != null) {
    return true;
  }

  if (!url || !url.includes('?')) {
    return false;
  }

  const rawQuery = url.split('?')[1] || '';
  const params = queryString.parse(rawQuery);
  return params.index != null;
}

function getNextIndexFromRequest(requestData = {}) {
  const index = getRequestIndex(requestData);
  if (index == null) {
    return null;
  }

  return String(index + 1);
}

function getRequestIndex(requestData = {}) {
  const { url = '', data } = requestData;

  if (data && typeof data === 'object' && data.index != null) {
    const parsed = Number(data.index);
    return Number.isFinite(parsed) ? parsed : null;
  }

  if (!url || !url.includes('?')) {
    return null;
  }

  const rawQuery = url.split('?')[1] || '';
  const params = queryString.parse(rawQuery);
  const parsed = Number(params.index);
  return Number.isFinite(parsed) ? parsed : null;
}

function lookupWatchList(adapter) {
  try {
    return adapter.watchList;
  } catch {
    const owner = getOwner(adapter);
    const isOwnerDestroyed = owner?.isDestroying || owner?.isDestroyed;

    if (isOwnerDestroyed) {
      return null;
    }

    try {
      return owner.lookup('service:watch-list');
    } catch {
      return null;
    }
  }
}

function lookupStore(adapter) {
  const owner = getOwner(adapter);
  const isOwnerDestroyed = owner?.isDestroying || owner?.isDestroyed;

  if (isOwnerDestroyed) {
    return null;
  }

  try {
    return owner.lookup('service:store');
  } catch {
    return null;
  }
}

function hasNonBlockingQueryParams(options) {
  if (!options || !options.data) return false;
  const keys = Object.keys(options.data);
  if (!keys.length) return false;
  if (keys.length === 1 && keys[0] === 'index') return false;

  return true;
}

function nextWatchIndex(index) {
  const parsedIndex = Number(index);
  const safeIndex = Number.isFinite(parsedIndex) ? parsedIndex : 1;
  return String(safeIndex + 1);
}

function shouldPreAdvanceWatchIndex() {
  return SHOULD_PRE_ADVANCE_WATCH_INDEX;
}
