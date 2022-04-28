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
        this.get('token.fetchSelfTokenAndPolicies')
          .perform()
          .catch();

        this.setProperties({
          tokenIsValid: true,
          tokenIsInvalid: false,
        });
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
}
