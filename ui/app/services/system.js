/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import { set } from '@ember/object';
import Service, { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import PromiseObject from '../utils/classes/promise-object';
import PromiseArray from '../utils/classes/promise-array';
import { namespace } from '../adapters/application';
import jsonWithDefault from '../utils/json-with-default';
import classic from 'ember-classic-decorator';
import { task } from 'ember-concurrency';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { tracked } from '@glimmer/tracking';

/**
 * @typedef {Object} RenderedDefaults
 * @property {string} [region]
 * @property {string[]} [namespace]
 * @property {string[]} [nodePool]
 */

@classic
export default class SystemService extends Service {
  @service token;
  @service store;

  /**
   * Iterates over all regions and returns a list of leaders' rpcAddrs
   */
  @computed('regions.[]')
  get leaders() {
    return Promise.all(
      this.regions.map((region) => {
        return this.token
          .authorizedRequest(`/${namespace}/status/leader?region=${region}`)
          .then((res) => res.json());
      })
    );
  }

  /**
   * @typedef {Object} Agent
   * @property {Config} config
   * @property {string} version
   */

  /**
   * @typedef {Object} Config
   * @property {UI} UI
   */

  /**
   * @typedef {Object} PeerApp
   * @property {string} BaseUIURL
   */

  /**
   * @typedef {Object} UILabel
   * @property {string} BackgroundColor
   * @property {string} Text
   * @property {string} TextColor
   */

  /**
   * @typedef {Object} Defaults
   * @property {string} Region
   * @property {string} Namespace
   * @property {string} NodePool
   */

  /**
   * @typedef {Object} UI
   * @property {UILabel} Label
   * @property {Defaults} Defaults
   * @property {PeerApp} Consul
   * @property {PeerApp} Vault
   * @property {boolean} Enabled
   * @property {Object} ContentSecurityPolicy
   */

  /**
   * Fetches the agent information for the current token
   * @type {Promise<Agent>}
   */
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

  @computed('token.selfToken')
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

  @localStorageProperty('nomadDefaultNamespace') userDefaultNamespace;
  @localStorageProperty('nomadDefaultNodePool') userDefaultNodePool;
  @localStorageProperty('nomadActiveRegion') userDefaultRegion;

  @tracked agentDefaults = {};

  /**
   * First read agent config for cluster-level defaults,
   * then check localStorageProperties for user-level overrides.
   * @type {Promise<RenderedDefaults>}
   */
  @computed(
    'agent',
    'agentDefaults.{Namespace,NodePool,Region}',
    'userDefaultNamespace',
    'userDefaultNodePool',
    'userDefaultRegion'
  )
  get defaults() {
    return this.agent.then((agent) => {
      /**
       * @type {Defaults}
       */
      // eslint-disable-next-line ember/no-side-effects
      this.agentDefaults = agent?.config?.UI?.Defaults || {};
      return {
        region: this.userDefaultRegion || this.agentDefaults.Region,
        namespace: (this.userDefaultNamespace || this.agentDefaults.Namespace)
          ?.split(',')
          .map((ns) => ns.trim()),
        nodePool: (this.userDefaultNodePool || this.agentDefaults.NodePool)
          ?.split(',')
          .map((np) => np.trim()),
      };
    });
  }

  @computed('regions.[]', 'userDefaultRegion')
  get activeRegion() {
    const regions = this.regions;
    const region = this.userDefaultRegion;

    if (regions.includes(region)) {
      return region;
    }

    return null;
  }

  set activeRegion(value) {
    if (value == null) {
      set(this, 'userDefaultRegion', null);
      return;
    } else {
      set(this, 'userDefaultRegion', value);
    }
  }

  @computed('regions.[]')
  get shouldShowRegions() {
    return this.get('regions.length') > 1;
  }

  get hasNonDefaultRegion() {
    return this.get('regions')
      .toArray()
      .some((region) => region !== 'global');
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

  get shouldShowNodepools() {
    return true; // TODO: make this dependent on there being at least one non-default node pool
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
