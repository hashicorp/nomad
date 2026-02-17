/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import Ember from 'ember';

@tagName('')
export default class ForbiddenMessage extends Component {
  @service token;
  @service store;
  @service router;

  get authMethods() {
    return this.store.findAll('auth-method');
  }

  generateNonce() {
    let randomArray = new Uint32Array(10);
    crypto.getRandomValues(randomArray);
    return randomArray.join('').slice(0, 20);
  }

  @action
  redirectToSSO(method, event) {
    event?.preventDefault();
    const provider = method.name;
    const nonce = this.generateNonce();

    window.localStorage.setItem('nomadOIDCNonce', nonce);
    window.localStorage.setItem('nomadOIDCAuthMethod', provider);

    let redirectURL;
    if (Ember.testing) {
      redirectURL = this.router.currentURL;
    } else {
      redirectURL = new URL(window.location.toString());
      redirectURL.search = '';
      redirectURL = redirectURL.href;
    }

    method
      .getAuthURL({
        AuthMethodName: provider,
        ClientNonce: nonce,
        RedirectUri: redirectURL,
      })
      .then(({ AuthURL }) => {
        if (Ember.testing) {
          this.router.transitionTo(AuthURL.split('/ui')[1]);
        } else {
          window.location = AuthURL;
        }
      });
  }
}
