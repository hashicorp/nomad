/* eslint-disable ember/no-observers */
import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { run } from '@ember/runloop';
import { observes } from '@ember-decorators/object';
import { computed } from '@ember/object';
import Ember from 'ember';
import codesForError from '../utils/codes-for-error';
import NoLeaderError from '../utils/no-leader-error';
import classic from 'ember-classic-decorator';

@classic
export default class ApplicationController extends Controller {
  @service config;
  @service system;

  queryParams = [
    {
      region: 'region',
    },
  ];

  region = null;

  error = null;

  @computed('error')
  get errorStr() {
    return this.error.toString();
  }

  @computed('error')
  get errorCodes() {
    return codesForError(this.error);
  }

  @computed('errorCodes.[]')
  get is403() {
    return this.errorCodes.includes('403');
  }

  @computed('errorCodes.[]')
  get is404() {
    return this.errorCodes.includes('404');
  }

  @computed('errorCodes.[]')
  get is500() {
    return this.errorCodes.includes('500');
  }

  @computed('error')
  get isNoLeader() {
    const error = this.error;
    return error instanceof NoLeaderError;
  }

  @observes('error')
  throwError() {
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
  }
}
