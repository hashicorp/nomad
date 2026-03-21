/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { set } from '@ember/object';
import Service, { service } from '@ember/service';
import PromiseObject from '../utils/classes/promise-object';
import PromiseArray from '../utils/classes/promise-array';
import { namespace } from '../adapters/application';
import jsonWithDefault from '../utils/json-with-default';
import { task } from 'ember-concurrency';
export default class SystemService extends Service {
  @service token;
  @service store;

  /**
   * Iterates over all regions and returns a list of leaders' rpcAddrs
   */
  get leaders() {
    return Promise.all(
      this.regions.map((region) => {
        return this.token
          .authorizedRequest(`/${namespace}/status/leader?region=${region}`)
          .then((res) => res.json());
      }),
    );
  }

  get agent() {
    const token = this.token;
    return PromiseObject.create({
      promise: token
        .authorizedRawRequest(`/${namespace}/agent/self`)
        .then(jsonWithDefault({}))
        .then((agent) => {
          if (agent?.config?.Version) {
            const { Version, VersionPrerelease, VersionMetadata } =
              agent.config.Version;
            agent.version = Version;
            if (VersionPrerelease)
              agent.version = `${agent.version}-${VersionPrerelease}`;
            if (VersionMetadata)
              agent.version = `${agent.version}+${VersionMetadata}`;
          }
          return agent;
        }),
    });
  }

  get defaultRegion() {
    const token = this.token;
    return PromiseObject.create({
      promise: token
        .authorizedRawRequest(`/${namespace}/agent/members`)
        .then(jsonWithDefault({}))
        .then((json) => {
          return { region: json.ServerRegion };
        }),
    });
  }

  get regions() {
    const token = this.token;

    return PromiseArray.create({
      promise: token
        .authorizedRawRequest(`/${namespace}/regions`)
        .then(jsonWithDefault([])),
    });
  }

  get activeRegion() {
    const regions = this.regions;
    const region = window.localStorage.nomadActiveRegion;

    if (regions.includes(region)) {
      return region;
    }

    return null;
  }

  set activeRegion(value) {
    if (value == null) {
      window.localStorage.removeItem('nomadActiveRegion');
      return;
    } else {
      // All localStorage values are strings. Stringify first so
      // the return value is consistent with what is persisted.
      const strValue = value + '';
      window.localStorage.nomadActiveRegion = strValue;
    }
  }

  get shouldShowRegions() {
    return (this.regions?.length || 0) > 1;
  }

  get hasNonDefaultRegion() {
    const regions = this.regions;
    const regionList =
      typeof regions?.toArray === 'function' ? regions.toArray() : regions;

    return (regionList || []).some((region) => region !== 'global');
  }

  get shouldIncludeRegion() {
    return (
      this.shouldShowRegions && this.activeRegion !== this.defaultRegion?.region
    );
  }

  get namespaces() {
    return PromiseArray.create({
      promise: this.store
        .findAll('namespace')
        .then((namespaces) => namespaces.compact()),
    });
  }

  get shouldShowNamespaces() {
    if (this._shouldShowNamespacesOverride !== undefined) {
      return this._shouldShowNamespacesOverride;
    }

    const namespaces = this.namespaces.toArray();
    return (
      namespaces.length &&
      namespaces.some((namespace) => namespace.get('id') !== 'default')
    );
  }

  set shouldShowNamespaces(value) {
    set(this, '_shouldShowNamespacesOverride', value);
  }

  get shouldShowNodepools() {
    return true; // TODO: make this dependent on there being at least one non-default node pool
  }

  @task(function* () {
    const emptyLicense = { License: { Features: [] } };

    try {
      return yield this.token
        .authorizedRawRequest(`/${namespace}/operator/license`)
        .then(jsonWithDefault(emptyLicense));
    } catch {
      return emptyLicense;
    }
  })
  fetchLicense;

  @task(function* () {
    try {
      const request = yield this.token.authorizedRequest('/v1/search/fuzzy', {
        method: 'POST',
        body: JSON.stringify({
          Text: 'feature-detection-query',
          Context: 'namespaces',
        }),
      });

      return request.ok;
    } catch {
      return false;
    }
  })
  checkFuzzySearchPresence;

  get license() {
    return this.fetchLicense.lastSuccessful?.value;
  }

  get fuzzySearchEnabled() {
    return this.checkFuzzySearchPresence.last?.value;
  }

  get features() {
    return this.license?.License?.Features || [];
  }
}
