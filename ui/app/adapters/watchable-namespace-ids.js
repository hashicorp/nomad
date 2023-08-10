/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Watchable from './watchable';
import classic from 'ember-classic-decorator';

@classic
export default class WatchableNamespaceIDs extends Watchable {
  @service system;

  findAll() {
    return super.findAll(...arguments).then((data) => {
      data.forEach((record) => {
        record.Namespace = 'default';
      });
      return data;
    });
  }

  query(store, type, { namespace }) {
    return super.query(...arguments).then((data) => {
      data.forEach((record) => {
        if (!record.Namespace) record.Namespace = namespace;
      });
      return data;
    });
  }

  findRecord(store, type, id, snapshot) {
    const [, namespace] = JSON.parse(id);
    const namespaceQuery =
      namespace && namespace !== 'default' ? { namespace } : {};

    return super.findRecord(store, type, id, snapshot, namespaceQuery);
  }

  urlForFindAll() {
    const url = super.urlForFindAll(...arguments);
    return associateNamespace(url);
  }

  urlForQuery() {
    const url = super.urlForQuery(...arguments);
    return associateNamespace(url);
  }

  urlForFindRecord(id, type, hash, pathSuffix) {
    const [name, namespace] = JSON.parse(id);
    let url = super.urlForFindRecord(name, type, hash);
    if (pathSuffix) url += `/${pathSuffix}`;
    return associateNamespace(url, namespace);
  }

  xhrKey(url, method, options = {}) {
    const plainKey = super.xhrKey(...arguments);
    const namespace = options.data && options.data.namespace;
    return associateNamespace(plainKey, namespace);
  }
}

function associateNamespace(url, namespace) {
  if (namespace && namespace !== 'default') {
    url += `?namespace=${namespace}`;
  }
  return url;
}
