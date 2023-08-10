/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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
    data.Policies = data.PolicyIDs;
    return this.ajax(`${this.buildURL()}/token`, 'POST', { data });
  }

  // Delete at /token instead of /tokens
  urlForDeleteRecord(identifier, modelName) {
    return `${this.buildURL()}/${singularize(modelName)}/${identifier}`;
  }

  async findSelf() {
    const response = await this.ajax(`${this.buildURL()}/token/self`, 'GET');
    const normalized = this.store.normalize('token', response);
    const tokenRecord = this.store.push(normalized);
    return tokenRecord;
  }

  async loginJWT(LoginToken, AuthMethodName) {
    const response = await this.ajax(`${this.buildURL()}/login`, 'POST', {
      data: {
        AuthMethodName,
        LoginToken,
      },
    });
    const normalized = this.store.normalize('token', response);
    const tokenRecord = this.store.push(normalized);
    return tokenRecord;
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
