// @ts-check
import { inject as service } from '@ember/service';
import { reads } from '@ember/object/computed';
import Controller from '@ember/controller';
import { getOwner } from '@ember/application';
import { alias } from '@ember/object/computed';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';
import { tracked } from '@glimmer/tracking';
import Ember from 'ember';

@classic
export default class Tokens extends Controller {
  @service token;
  @service store;
  @service router;

  queryParams = ['code', 'state'];

  @reads('token.secret') secret;

  /**
   * @type {(null | "success" | "failure")} signInStatus
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

  get authMethods() {
    return this.store.peekAll('auth-method');
  }

  @action
  verifyToken() {
    const { secret } = this;
    this.clearTokenProperties();
    const TokenAdapter = getOwner(this).lookup('adapter:token');

    this.set('token.secret', secret);
    this.set('secret', null);

    TokenAdapter.findSelf().then(
      () => {
        // Clear out all data to ensure only data the new token is privileged to see is shown
        this.resetStore();

        // Refetch the token and associated policies
        this.get('token.fetchSelfTokenAndPolicies').perform().catch();

        this.signInStatus = 'success';
        this.token.set('tokenNotFound', false);
      },
      () => {
        this.set('token.secret', undefined);
        this.signInStatus = 'failure';
      }
    );
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

      this.resetStore();

      // Refetch the token and associated policies
      this.get('token.fetchSelfTokenAndPolicies').perform().catch();

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
