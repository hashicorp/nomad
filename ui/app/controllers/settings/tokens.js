import { inject as service } from '@ember/service';
import { reads } from '@ember/object/computed';
import Controller from '@ember/controller';
import { getOwner } from '@ember/application';
import { alias } from '@ember/object/computed';

export default Controller.extend({
  token: service(),
  system: service(),
  store: service(),

  secret: reads('token.secret'),

  tokenIsValid: false,
  tokenIsInvalid: false,
  tokenRecord: alias('token.selfToken'),

  resetStore() {
    this.store.unloadAll();
  },

  actions: {
    clearTokenProperties() {
      this.token.setProperties({
        secret: undefined,
      });
      this.setProperties({
        tokenIsValid: false,
        tokenIsInvalid: false,
      });
      this.resetStore();
      this.token.reset();
    },

    verifyToken() {
      const { secret } = this;
      const TokenAdapter = getOwner(this).lookup('adapter:token');

      this.set('token.secret', secret);

      TokenAdapter.findSelf().then(
        () => {
          // Clear out all data to ensure only data the new token is privileged to
          // see is shown
          this.system.reset();
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
    },
  },
});
