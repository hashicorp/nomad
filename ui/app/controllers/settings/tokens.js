// @ts-check
import { inject as service } from '@ember/service';
import { reads } from '@ember/object/computed';
import Controller from '@ember/controller';
import { getOwner } from '@ember/application';
import { alias } from '@ember/object/computed';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';
import { tracked } from '@glimmer/tracking';

@classic
export default class Tokens extends Controller {
  @service token;
  @service store;

  queryParams = ['code', 'state'];

  @reads('token.secret') secret;

  tokenIsValid = false;
  tokenIsInvalid = false;
  SSOFailure = false;
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
    this.setProperties({
      tokenIsValid: false,
      tokenIsInvalid: false,
    });
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
    const TokenAdapter = getOwner(this).lookup('adapter:token');

    this.set('token.secret', secret);

    TokenAdapter.findSelf().then(
      () => {
        // Clear out all data to ensure only data the new token is privileged to see is shown
        this.resetStore();

        // Refetch the token and associated policies
        this.get('token.fetchSelfTokenAndPolicies').perform().catch();

        this.setProperties({
          tokenIsValid: true,
          tokenIsInvalid: false,
          SSOFailure: false,
        });
        this.token.set('tokenNotFound', false);
      },
      () => {
        this.set('token.secret', undefined);
        this.setProperties({
          tokenIsValid: false,
          tokenIsInvalid: true,
        });
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

    method
      .getAuthURL({
        AuthMethod: provider,
        ClientNonce: nonce,
        RedirectUri: window.location.toString(), // TODO: decide if you want them back on /tokens.
      })
      .then(({ AuthURL }) => {
        window.location = AuthURL;
      });
  }

  @tracked code = null;
  @tracked state = null;

  get isValidatingToken() {
    if (this.code && this.state === 'success') {
      this.validateSSO();
      return true;
    } else {
      if (this.state === 'failure') {
        this.SSOFailure = true;
      }
      return false;
    }
  }

  validateSSO() {
    this.token
      .authorizedRequest('/v1/acl/oidc/complete-auth', {
        method: 'POST',
        body: JSON.stringify({
          AuthMethod: window.localStorage.getItem('nomadOIDCAuthMethod'),
          ClientNonce: window.localStorage.getItem('nomadOIDCNonce'),
          Code: this.code,
          State: this.state,
        }),
      })
      .then(async (response) => {
        if (response.ok) {
          let json = await response.json();
          this.token.set('secret', json.ACLToken);
          this.verifyToken();
          this.code = null;
          this.state = null;
        }
      });
  }
}
