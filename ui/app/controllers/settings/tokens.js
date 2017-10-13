import Ember from 'ember';

const { Controller, inject, computed } = Ember;

export default Controller.extend({
  token: inject.service(),

  tokenRecord: null,
  secret: computed.reads('token.secret'),
  accessor: computed.reads('token.accessor'),

  tokenIsValid: false,
  tokenIsInvalid: false,

  actions: {
    clearTokenProperties() {
      this.get('token').setProperties({
        secret: undefined,
        accessor: undefined,
      });
      this.setProperties({
        tokenIsValid: false,
        tokenIsInvalid: false,
      });
    },

    verifyToken() {
      const { secret, accessor } = this.getProperties('secret', 'accessor');

      this.set('token.secret', secret);
      this.get('store')
        .findRecord('token', accessor)
        .then(
          token => {
            this.set('token.accessor', accessor);
            this.setProperties({
              tokenIsValid: true,
              tokenIsInvalid: false,
              tokenRecord: token,
            });
          },
          () => {
            this.set('token.secret', null);
            this.setProperties({
              tokenIsInvalid: true,
              tokenIsValid: false,
              tokenRecord: null,
            });
          }
        );
    },
  },
});
