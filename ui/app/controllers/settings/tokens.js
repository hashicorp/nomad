import { inject as service } from '@ember/service';
import { reads } from '@ember/object/computed';
import Controller from '@ember/controller';
import { getOwner } from '@ember/application';

export default Controller.extend({
  token: service(),
  system: service(),
  store: service(),

  secret: reads('token.secret'),

  tokenIsValid: false,
  tokenIsInvalid: false,
  tokenRecord: null,

  resetStore() {
    this.get('store').unloadAll();
  },

  actions: {
    clearTokenProperties() {
      this.get('token').setProperties({
        secret: undefined,
      });
      this.setProperties({
        tokenIsValid: false,
        tokenIsInvalid: false,
        tokenRecord: null,
      });
      this.resetStore();
    },

    verifyToken() {
      const { secret } = this.getProperties('secret', 'accessor');
      const TokenAdapter = getOwner(this).lookup('adapter:token');

      this.set('token.secret', secret);

      TokenAdapter.findSelf().then(
        token => {
          // Capture the token ID before clearing the store
          const tokenId = token.get('id');

          // Clear out all data to ensure only data the new token is privileged to
          // see is shown
          this.get('system').reset();
          this.resetStore();

          // Immediately refetch the token now that the store is empty
          const newToken = this.get('store').findRecord('token', tokenId);

          this.setProperties({
            tokenIsValid: true,
            tokenIsInvalid: false,
            tokenRecord: newToken,
          });
        },
        () => {
          this.set('token.secret', undefined);
          this.setProperties({
            tokenIsValid: false,
            tokenIsInvalid: true,
            tokenRecord: null,
          });
        }
      );
    },
  },
});
