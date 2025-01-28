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
  @service can;

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

  @computed(
    'defaults',
    'token.selfToken',
    'variableDefaults.Region',
    'regions.[]'
  )
  get defaultRegion() {
    // 1. If there is a defaults.region, and that region is within this.regions, return that region.
    // 2. Otherwise, fallback to the agent/members' json.ServerRegion
    const token = this.token;
    const regions = this.regions;
    return this.defaults.then((defaults) => {
      if (defaults.region && regions.includes(defaults.region)) {
        return { region: defaults.region };
      } else {
        return PromiseObject.create({
          promise: token
            .authorizedRawRequest(`/${namespace}/agent/members`)
            .then(jsonWithDefault({}))
            .then((json) => {
              return { region: json.ServerRegion };
            }),
        });
      }
    });
  }

  defaultProperties = [
    'userDefaultRegion',
    'userDefaultNamespace',
    'userDefaultNodePool',
  ];

  async establishUIDefaults() {
    // // First, check to see if there are localStorage properties set for each of the defaults.
    // // If there are, we don't need to reach out to check the variable or agent config.
    // if (this.defaultProperties.every((defaultString) => this[defaultString])) {
    //   return;
    // }

    let agent = await this.agent;
    this.agentDefaults = agent?.config?.UI?.Defaults || {};
    let variableDefaults = await this.fetchVariableDefaults();
    this.variableDefaults = variableDefaults || {};
    return {
      agentDefaults: this.agentDefaults,
      variableDefaults: this.variableDefaults,
    };
  }

  async fetchVariableDefaults() {
    try {
      // if (this.can.can('read variable', 'nomad/ui/defaults', '*')) { // TODO: is wildcard correctly handled by "can"?

      // Get all variables in defaults across all namespaces
      const variables = await this.store.query('variable', {
        prefix: 'nomad/ui/defaults',
        namespace: '*',
      });
      // If any exist, take the first one and read it
      const firstVariable = variables.firstObject;
      if (firstVariable) {
        const variableDefaults = await this.store.findRecord(
          'variable',
          `${firstVariable.path}@${firstVariable.namespace}`,
          { reload: true }
        );
        return {
          Namespace: variableDefaults.items.namespace,
          NodePool: variableDefaults.items.nodepool,
          Region: variableDefaults.items.region,
        };
      } else {
        return null;
      }
      // }
    } catch (e) {
      console.warn('Failed to load UI defaults:', e);
      return null;
    }
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
  @tracked variableDefaults = {};

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
    'userDefaultRegion',
    'variableDefaults.{Namespace,NodePool,Region}'
  )
  get defaults() {
    /**
     * @type {Defaults}
     */
    return new Promise((resolve) => {
      resolve({
        region:
          this.userDefaultRegion || // from localStorage
          this.variableDefaults.Region || // from variable defaults
          this.agentDefaults.Region, // from agent config
        namespace: (
          this.userDefaultNamespace ||
          this.variableDefaults.Namespace || // TODO: probably have to split/map/trim variableDefaults, too.
          this.agentDefaults.Namespace
        )
          ?.split(',')
          .map((ns) => ns.trim()),
        nodePool: (
          this.userDefaultNodePool ||
          this.variableDefaults.NodePool ||
          this.agentDefaults.NodePool
        )
          ?.split(',')
          .map((np) => np.trim()),
      });
    });
  }

  @task(function* () {
    const [regions, defaultRegion] = yield Promise.all([
      this.regions,
      this.defaultRegion,
    ]);
    if (regions.includes(defaultRegion.region)) {
      if (regions.includes(defaultRegion.region)) {
        return defaultRegion.region;
      }
    } else {
      return null;
    }
  })
  computeActiveRegion;

  @computed('computeActiveRegion.lastSuccessful.value')
  get activeRegion() {
    // Get the last successful run's value, if it exists
    if (this.computeActiveRegion.lastSuccessful) {
      return this.computeActiveRegion.lastSuccessful.value;
    }

    // If we haven't run yet, kick off the task and return null for now
    this.computeActiveRegion.perform();
    return null;
  }

  set activeRegion(value) {
    if (value == null) {
      set(this, 'userDefaultRegion', null);
      return;
    } else {
      set(this, 'userDefaultRegion', value);
      this.computeActiveRegion.perform();
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
