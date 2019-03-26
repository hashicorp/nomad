import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { run } from '@ember/runloop';
import { observer, computed } from '@ember/object';
import Ember from 'ember';
import codesForError from '../utils/codes-for-error';
import NoLeaderError from '../utils/no-leader-error';

export default Controller.extend({
  config: service(),
  system: service(),

  queryParams: {
    region: 'region',
  },

  region: null,

  error: null,

  errorStr: computed('error', function() {
    return this.error.toString();
  }),

  errorCodes: computed('error', function() {
    return codesForError(this.error);
  }),

  is403: computed('errorCodes.[]', function() {
    return this.errorCodes.includes('403');
  }),

  is404: computed('errorCodes.[]', function() {
    return this.errorCodes.includes('404');
  }),

  is500: computed('errorCodes.[]', function() {
    return this.errorCodes.includes('500');
  }),

  isNoLeader: computed('error', function() {
    const error = this.error;
    return error instanceof NoLeaderError;
  }),

  throwError: observer('error', function() {
    if (this.get('config.isDev')) {
      run.next(() => {
        throw this.error;
      });
    } else if (!Ember.testing) {
      run.next(() => {
        // eslint-disable-next-line
        console.warn('UNRECOVERABLE ERROR:', this.error);
      });
    }
  }),
});
