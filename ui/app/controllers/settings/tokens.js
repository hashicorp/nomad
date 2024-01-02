/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { getOwner } from '@ember/application';
import { alias } from '@ember/object/computed';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';
import { tracked } from '@glimmer/tracking';
import Ember from 'ember';

/**
 * @type {RegExp}
 */
const JWT_MATCH_EXPRESSION = /^[a-zA-Z0-9-_]+\.[a-zA-Z0-9-_]+\.[a-zA-Z0-9-_]+$/;

@classic
export default class Tokens extends Controller {
  @service token;
  @service store;
  @service router;

  queryParams = ['code', 'state', 'jwtAuthMethod'];

  @tracked secret = this.token.secret;

  /**
   * @type {(null | "success" | "failure" | "jwtFailure")} signInStatus
   */
  @tracked
  signInStatus = null;

  @alias('token.selfToken') tokenRecord;

  resetStore() {
    this.store.unloadAll();
  }

  @action
  clearTokenProperties() {
    this.token.setProperties({
      secret: undefined,
      tokenNotFound: false,
    });
    this.signInStatus = null;
    // Clear out all data to ensure only data the anonymous token is privileged to see is shown
    this.resetStore();
    this.token.reset();
    this.store.findAll('auth-method');
  }

  /**
   * @returns {import('@ember/array/mutable').default<import('../../models/auth-method').default>}
   */
  get authMethods() {
    return this.model?.authMethods || [];
  }

  get hasJWTAuthMethods() {
    return this.authMethods.any((method) => method.type === 'JWT');
  }

  get nonTokenAuthMethods() {
    return this.authMethods.rejectBy('type', 'JWT');
  }

  get JWTAuthMethods() {
    return this.authMethods.filterBy('type', 'JWT');
  }

  get JWTAuthMethodOptions() {
    return this.JWTAuthMethods.map((method) => ({
      key: method.name,
      label: method.name,
    }));
  }

  get defaultJWTAuthMethod() {
    return (
      this.JWTAuthMethods.findBy('default', true) || this.JWTAuthMethods[0]
    );
  }

  @action
  setCurrentAuthMethod() {
    if (!this.jwtAuthMethod) {
      this.jwtAuthMethod = this.defaultJWTAuthMethod?.name;
    }
  }

  /**
   * @type {string}
   */
  @tracked jwtAuthMethod;

  /**
   * @type {boolean}
   */
  get currentSecretIsJWT() {
    return this.secret?.length > 36 && this.secret.match(JWT_MATCH_EXPRESSION);
  }

  @action
  async verifyToken() {
    const { secret } = this;
    /**
     * @type {import('../../adapters/token').default}
     */

    // Ember currently lacks types for getOwner: https://github.com/emberjs/ember.js/issues/19916
    // @ts-ignore
    const TokenAdapter = getOwner(this).lookup('adapter:token');

    const isJWT = secret.length > 36 && secret.match(JWT_MATCH_EXPRESSION);

    if (isJWT) {
      const methodName = this.jwtAuthMethod;

      // If user passes a JWT token, but there is no JWT auth method, throw an error
      if (!methodName) {
        this.token.set('secret', undefined);
        this.signInStatus = 'jwtFailure';
        return;
      }

      this.clearTokenProperties();

      // Set bearer token instead of findSelf etc.
      TokenAdapter.loginJWT(secret, methodName).then(
        (token) => {
          this.token.setProperties({
            secret: token.secret,
            tokenNotFound: false,
          });
          this.set('secret', null);

          // Clear out all data to ensure only data the new token is privileged to see is shown
          this.resetStore();

          // Refetch the token and associated policies
          this.token.get('fetchSelfTokenAndPolicies').perform().catch();

          this.signInStatus = 'success';
        },
        () => {
          this.token.set('secret', undefined);
          this.signInStatus = 'failure';
        }
      );
    } else {
      this.clearTokenProperties();
      this.token.set('secret', secret);
      this.set('secret', null);

      TokenAdapter.findSelf().then(
        () => {
          // Clear out all data to ensure only data the new token is privileged to see is shown
          this.resetStore();

          // Refetch the token and associated policies
          this.token.get('fetchSelfTokenAndPolicies').perform().catch();

          this.signInStatus = 'success';
          this.token.set('tokenNotFound', false);
        },
        () => {
          this.token.set('secret', undefined);
          this.signInStatus = 'failure';
        }
      );
    }
  }

  // Generate a 20-char nonce, using window.crypto to
  // create a sufficiently-large output then trimming
  generateNonce() {
    let randomArray = new Uint32Array(10);
    crypto.getRandomValues(randomArray);
    return randomArray.join('').slice(0, 20);
  }

  @action redirectToSSO(method) {
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

  @tracked code = null;
  @tracked state = null;

  get isValidatingToken() {
    if (this.code && this.state) {
      this.validateSSO();
      return true;
    } else {
      return false;
    }
  }

  async validateSSO() {
    let redirectURL;
    if (Ember.testing) {
      redirectURL = this.router.currentURL;
    } else {
      redirectURL = new URL(window.location.toString());
      redirectURL.search = '';
      redirectURL = redirectURL.href;
    }

    const res = await this.token.authorizedRequest(
      '/v1/acl/oidc/complete-auth',
      {
        method: 'POST',
        body: JSON.stringify({
          AuthMethodName: window.localStorage.getItem('nomadOIDCAuthMethod'),
          ClientNonce: window.localStorage.getItem('nomadOIDCNonce'),
          Code: this.code,
          State: this.state,
          RedirectURI: redirectURL,
        }),
      }
    );

    if (res.ok) {
      const data = await res.json();
      this.clearTokenProperties();
      this.token.set('secret', data.SecretID);
      this.state = null;
      this.code = null;

      // Refetch the token and associated policies
      this.token.get('fetchSelfTokenAndPolicies').perform().catch();

      this.signInStatus = 'success';
      this.token.set('tokenNotFound', false);
    } else {
      this.state = 'failure';
      this.code = null;
    }
  }

  get SSOFailure() {
    return this.state === 'failure';
  }

  get canSignIn() {
    return !this.tokenRecord || this.tokenRecord.isExpired;
  }

  get shouldShowPolicies() {
    return this.tokenRecord;
  }
}
