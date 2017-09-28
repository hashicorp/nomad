import Ember from 'ember';

const { Controller, computed } = Ember;

export default Controller.extend({
  error: null,

  errorStr: computed('error', function() {
    return this.get('error').toString();
  }),

  errorCodes: computed('error', function() {
    const error = this.get('error');
    const codes = [error.code];

    if (error.errors) {
      error.errors.forEach(err => {
        codes.push(err.status);
      });
    }

    return codes
      .compact()
      .uniq()
      .map(code => '' + code);
  }),

  is404: computed('errorCodes.[]', function() {
    return this.get('errorCodes').includes('404');
  }),

  is500: computed('errorCodes.[]', function() {
    return this.get('errorCodes').includes('500');
  }),
});
