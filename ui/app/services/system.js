import Ember from 'ember';
import PromiseObject from '../utils/classes/promise-object';
import { namespace } from '../adapters/application';

const { Service, computed, inject } = Ember;

export default Service.extend({
  token: inject.service(),
  store: inject.service(),

  leader: computed(function() {
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

  namespaces: computed(function() {
    return this.get('store').findAll('namespace');
  }),

  shouldShowNamespaces: computed('namespaces.[]', function() {
    const namespaces = this.get('namespaces').toArray();
    return namespaces.length && namespaces.some(namespace => namespace.get('id') !== 'default');
  }),

  activeNamespace: computed('namespaces.[]', {
    get() {
      const namespaceId = window.localStorage.nomadActiveNamespace || 'default';
      return this.get('namespaces').findBy('id', namespaceId);
    },
    set(key, value) {
      if (value == null) {
        window.localStorage.removeItem('nomadActiveNamespace');
      } else if (typeof value === 'string') {
        window.localStorage.nomadActiveNamespace = value;
        return this.get('namespaces').findBy('id', value);
      }
      window.localStorage.nomadActiveNamespace = value.get('name');
      return value;
    },
  }),
});
