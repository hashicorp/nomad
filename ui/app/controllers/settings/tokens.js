// @ts-check
import { inject as service } from '@ember/service';
import { reads } from '@ember/object/computed';
import Controller from '@ember/controller';
import { getOwner } from '@ember/application';
import { alias } from '@ember/object/computed';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class Tokens extends Controller {
  @service token;
  @service store;

  @reads('token.secret') secret;

  tokenIsValid = false;
  tokenIsInvalid = false;
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

  @action signInWithSSO(method) {
    const provider = method.name;
    const nonce = this.generateNonce();

    window.localStorage.setItem('nomadOIDCNonce', nonce);
    window.localStorage.setItem('nomadOIDCAuthMethod', provider);

    let returned = method
      .getAuthURL({
        AuthMethod: provider,
        ClientNonce: nonce,
        RedirectUri: window.location.toString(), // TODO: decide if you want them back on /tokens.
      })
      .then(({ AuthURL }) => {
        console.log('AUTHURL BACK', AuthURL);
        window.location = AuthURL;
      });
    console.log('returned', returned);
  }
}
