import { inject as service } from '@ember/service';
import { default as ApplicationAdapter, namespace } from './application';
import OTTExchangeError from '../utils/ott-exchange-error';
import classic from 'ember-classic-decorator';
import { singularize } from 'ember-inflector';

@classic
export default class TokenAdapter extends ApplicationAdapter {
  @service store;

  namespace = namespace + '/acl';

  createRecord(_store, type, snapshot) {
    let data = this.serialize(snapshot);
    console.log('DATA GOING OUT', data);
    data.Policies = data.PolicyIDs; // TODO: temp hack
    return this.ajax(`${this.buildURL()}/token`, 'POST', { data });
  }

  findSelf() {
    return this.ajax(`${this.buildURL()}/token/self`, 'GET').then((token) => {
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
    })
      .then(({ Token: token }) => {
        const store = this.store;
        store.pushPayload('token', {
          tokens: [token],
        });

        return store.peekRecord(
          'token',
          store.normalize('token', token).data.id
        );
      })
      .catch(() => {
        throw new OTTExchangeError();
      });
  }
}
