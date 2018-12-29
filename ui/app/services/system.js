import Service, { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { copy } from '@ember/object/internals';
import PromiseObject from '../utils/classes/promise-object';
import PromiseArray from '../utils/classes/promise-array';
import { namespace } from '../adapters/application';

// When the request isn't ok (e.g., forbidden) handle gracefully
const jsonWithDefault = defaultResponse => res =>
  res.ok ? res.json() : copy(defaultResponse, true);

export default Service.extend({
  token: service(),
  store: service(),

  leader: computed('activeRegion', function() {
    const token = this.get('token');

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
  }),

  defaultRegion: computed(function() {
    const token = this.get('token');
    return PromiseObject.create({
      promise: token
        .authorizedRawRequest(`/${namespace}/agent/members`)
        .then(jsonWithDefault({}))
        .then(json => {
          return { region: json.ServerRegion };
        }),
    });
  }),

  regions: computed(function() {
    const token = this.get('token');

    return PromiseArray.create({
      promise: token.authorizedRawRequest(`/${namespace}/regions`).then(jsonWithDefault([])),
    });
  }),

  activeRegion: computed('regions.[]', {
    get() {
      const regions = this.get('regions');
      const region = window.localStorage.nomadActiveRegion;

      if (regions.includes(region)) {
        return region;
      }

      return null;
    },
    set(key, value) {
      if (value == null) {
        window.localStorage.removeItem('nomadActiveRegion');
      } else {
        // All localStorage values are strings. Stringify first so
        // the return value is consistent with what is persisted.
        const strValue = value + '';
        window.localStorage.nomadActiveRegion = strValue;
        return strValue;
      }
    },
  }),

  shouldShowRegions: computed('regions.[]', function() {
    return this.get('regions.length') > 1;
  }),

  shouldIncludeRegion: computed(
    'activeRegion',
    'defaultRegion.region',
    'shouldShowRegions',
    function() {
      return (
        this.get('shouldShowRegions') &&
        this.get('activeRegion') !== this.get('defaultRegion.region')
      );
    }
  ),

  namespaces: computed('activeRegion', function() {
    return PromiseArray.create({
      promise: this.get('store')
        .findAll('namespace')
        .then(namespaces => namespaces.compact()),
    });
  }),

  shouldShowNamespaces: computed('namespaces.[]', function() {
    const namespaces = this.get('namespaces').toArray();
    return namespaces.length && namespaces.some(namespace => namespace.get('id') !== 'default');
  }),

  activeNamespace: computed('namespaces.[]', {
    get() {
      const namespaceId = window.localStorage.nomadActiveNamespace || 'default';
      const namespace = this.get('namespaces').findBy('id', namespaceId);

      if (namespace) {
        return namespace;
      }

      // If the namespace in localStorage is no longer in the cluster, it needs to
      // be cleared from localStorage
      this.set('activeNamespace', null);
      return this.get('namespaces').findBy('id', 'default');
    },
    set(key, value) {
      if (value == null) {
        window.localStorage.removeItem('nomadActiveNamespace');
      } else if (typeof value === 'string') {
        window.localStorage.nomadActiveNamespace = value;
        return this.get('namespaces').findBy('id', value);
      } else {
        window.localStorage.nomadActiveNamespace = value.get('name');
        return value;
      }
    },
  }),

  reset() {
    this.set('activeNamespace', null);
  },
});
