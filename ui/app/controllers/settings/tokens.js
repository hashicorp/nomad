import Ember from 'ember';

const { Controller, inject, computed, getOwner } = Ember;

export default Controller.extend({
  token: inject.service(),
  store: inject.service(),

  secret: computed.reads('token.secret'),

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
