import Ember from 'ember';
import codesForError from '../utils/codes-for-error';

const { Controller, computed, inject, run, observer } = Ember;

export default Controller.extend({
  config: inject.service(),

  error: null,

  errorStr: computed('error', function() {
    return this.get('error').toString();
  }),

  errorCodes: computed('error', function() {
    return codesForError(this.get('error'));
  }),

  is403: computed('errorCodes.[]', function() {
    return this.get('errorCodes').includes('403');
  }),

  is404: computed('errorCodes.[]', function() {
    return this.get('errorCodes').includes('404');
  }),

  is500: computed('errorCodes.[]', function() {
    return this.get('errorCodes').includes('500');
  }),

  throwError: observer('error', function() {
    if (this.get('config.isDev')) {
      run.next(() => {
        throw this.get('error');
      });
    } else {
      run.next(() => {
        // eslint-disable-next-line
        console.warn('UNRECOVERABLE ERROR:', this.get('error'));
      });
    }
  }),
});
