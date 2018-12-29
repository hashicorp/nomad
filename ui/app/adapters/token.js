import { inject as service } from '@ember/service';
import { default as ApplicationAdapter, namespace } from './application';

export default ApplicationAdapter.extend({
  store: service(),

  namespace: namespace + '/acl',

  findSelf() {
    return this.ajax(`${this.buildURL()}/token/self`, 'GET').then(token => {
      const store = this.get('store');
      store.pushPayload('token', {
        tokens: [token],
      });

      return store.peekRecord('token', store.normalize('token', token).data.id);
    });
  },
});
