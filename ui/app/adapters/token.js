import Ember from 'ember';
import { default as ApplicationAdapter, namespace } from './application';

const { inject } = Ember;

export default ApplicationAdapter.extend({
  store: inject.service(),

  namespace: namespace + '/acl',

  findSelf() {
    return this.ajax(`${this.buildURL()}/token/self`).then(token => {
      const store = this.get('store');
      store.pushPayload('token', {
        tokens: [token],
      });

      return store.peekRecord('token', store.normalize('token', token).data.id);
    });
  },
});
