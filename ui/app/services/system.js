/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service, { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import PromiseObject from '../utils/classes/promise-object';
import PromiseArray from '../utils/classes/promise-array';
import { namespace } from '../adapters/application';
import jsonWithDefault from '../utils/json-with-default';
import classic from 'ember-classic-decorator';
import { task } from 'ember-concurrency';

@classic
export default class SystemService extends Service {
  @service token;
  @service store;

  @computed('activeRegion')
  get leader() {
    const token = this.token;

    return PromiseObject.create({
      promise: token
        .authorizedRequest(`/${namespace}/status/leader`)
        .then((res) => res.json())
        .then((rpcAddr) => ({ rpcAddr }))
        .then((leader) => {
          // Dirty self so leader can be used as a dependent key
          this.notifyPropertyChange('leader.rpcAddr');
          return leader;
        }),
    });
  }

  @computed
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

  @computed
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

  @computed
  get regions() {
    const token = this.token;

    return PromiseArray.create({
      promise: token
        .authorizedRawRequest(`/${namespace}/regions`)
        .then(jsonWithDefault([])),
    });
  }

  @computed('regions.[]')
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

  @computed('regions.[]')
  get shouldShowRegions() {
    return this.get('regions.length') > 1;
  }

  @computed('activeRegion', 'defaultRegion.region', 'shouldShowRegions')
  get shouldIncludeRegion() {
    return (
      this.shouldShowRegions &&
      this.activeRegion !== this.get('defaultRegion.region')
    );
  }

  @computed('activeRegion')
  get namespaces() {
    return PromiseArray.create({
      promise: this.store
        .findAll('namespace')
        .then((namespaces) => namespaces.compact()),
    });
  }

  @computed('namespaces.[]')
  get shouldShowNamespaces() {
    const namespaces = this.namespaces.toArray();
    return (
      namespaces.length &&
      namespaces.some((namespace) => namespace.get('id') !== 'default')
    );
  }

  @task(function* () {
    const emptyLicense = { License: { Features: [] } };

    try {
      return yield this.token
        .authorizedRawRequest(`/${namespace}/operator/license`)
        .then(jsonWithDefault(emptyLicense));
    } catch (e) {
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
    } catch (e) {
      return false;
    }
  })
  checkFuzzySearchPresence;

  @alias('fetchLicense.lastSuccessful.value') license;
  @alias('checkFuzzySearchPresence.last.value') fuzzySearchEnabled;

  @computed('license.License.Features.[]')
  get features() {
    return this.get('license.License.Features') || [];
  }
}
