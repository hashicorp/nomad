import Service, { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import PromiseObject from '../utils/classes/promise-object';
import PromiseArray from '../utils/classes/promise-array';
import { namespace } from '../adapters/application';
import jsonWithDefault from '../utils/json-with-default';
import classic from 'ember-classic-decorator';

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
        .then(res => res.json())
        .then(rpcAddr => ({ rpcAddr }))
        .then(leader => {
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
        .then(agent => {
          agent.version = agent.member?.Tags?.build || 'Unknown';
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
        .then(json => {
          return { region: json.ServerRegion };
        }),
    });
  }

  @computed
  get regions() {
    const token = this.token;

    return PromiseArray.create({
      promise: token.authorizedRawRequest(`/${namespace}/regions`).then(jsonWithDefault([])),
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
      return strValue;
    }
  }

  @computed('regions.[]')
  get shouldShowRegions() {
    return this.get('regions.length') > 1;
  }

  @computed('activeRegion', 'defaultRegion.region', 'shouldShowRegions')
  get shouldIncludeRegion() {
    return this.shouldShowRegions && this.activeRegion !== this.get('defaultRegion.region');
  }

  @computed('activeRegion')
  get namespaces() {
    return PromiseArray.create({
      promise: this.store.findAll('namespace').then(namespaces => namespaces.compact()),
    });
  }

  @computed('namespaces.[]')
  get shouldShowNamespaces() {
    const namespaces = this.namespaces.toArray();
    return namespaces.length && namespaces.some(namespace => namespace.get('id') !== 'default');
  }

  @computed('namespaces.[]')
  get activeNamespace() {
    const namespaceId = window.localStorage.nomadActiveNamespace || 'default';
    const namespace = this.namespaces.findBy('id', namespaceId);

    if (namespace) {
      return namespace;
    }

    // If the namespace in localStorage is no longer in the cluster, it needs to
    // be cleared from localStorage
    window.localStorage.removeItem('nomadActiveNamespace');
    return this.namespaces.findBy('id', 'default');
  }

  set activeNamespace(value) {
    if (value == null) {
      window.localStorage.removeItem('nomadActiveNamespace');
      return;
    } else if (typeof value === 'string') {
      window.localStorage.nomadActiveNamespace = value;
      return this.namespaces.findBy('id', value);
    } else {
      window.localStorage.nomadActiveNamespace = value.get('name');
      return value;
    }
  }

  reset() {
    this.set('activeNamespace', null);
    this.notifyPropertyChange('namespaces');
  }
}
