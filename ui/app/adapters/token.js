import { inject as service } from '@ember/service';
import { default as ApplicationAdapter, namespace } from './application';
import OTTExchangeError from '../utils/ott-exchange-error';

export default class TokenAdapter extends ApplicationAdapter {
  @service store;

  namespace = namespace + '/acl';

  findSelf() {
    return this.ajax(`${this.buildURL()}/token/self`, 'GET').then(token => {
      const store = this.store;
      store.pushPayload('token', {
        tokens: [token],
      });

      return store.peekRecord('token', store.normalize('token', token).data.id);
    });
  }

  exchangeOneTimeToken(oneTimeToken) {
    return this.ajax(`${this.buildURL()}/token/onetime/exchange`, 'POST', {
      data: {
        OneTimeSecretID: oneTimeToken,
      },
    }).then(({ Token: token }) => {
      const store = this.store;
      store.pushPayload('token', {
        tokens: [token],
      });

      return store.peekRecord('token', store.normalize('token', token).data.id);
    }).catch(() => {
      throw new OTTExchangeError();
    });
  }
}
