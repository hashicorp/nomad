import Ember from 'ember';

const { Controller, inject, computed, getOwner } = Ember;

export default Controller.extend({
  token: inject.service(),

  secret: computed.reads('token.secret'),

  tokenIsValid: false,
  tokenIsInvalid: false,
  tokenRecord: null,

  actions: {
    clearTokenProperties() {
      this.get('token').setProperties({
        secret: undefined,
        accessor: undefined,
      });
      this.setProperties({
        tokenIsValid: false,
        tokenIsInvalid: false,
        tokenRecord: null,
      });
    },

    verifyToken() {
      const { secret } = this.getProperties('secret', 'accessor');
      const TokenAdapter = getOwner(this).lookup('adapter:token');

      this.set('token.secret', secret);

      TokenAdapter.findSelf().then(
        token => {
          this.setProperties({
            tokenIsValid: true,
            tokenIsInvalid: false,
            tokenRecord: token,
          });
        },
        () => {
          this.set('token.secret', null);
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
