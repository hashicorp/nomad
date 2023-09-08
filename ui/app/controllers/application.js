/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-observers */
import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { next } from '@ember/runloop';
import { observes } from '@ember-decorators/object';
import { computed } from '@ember/object';
import Ember from 'ember';
import codesForError from '../utils/codes-for-error';
import NoLeaderError from '../utils/no-leader-error';
import OTTExchangeError from '../utils/ott-exchange-error';
import classic from 'ember-classic-decorator';
// eslint-disable-next-line no-unused-vars
import KeyboardService from '../services/keyboard';
@classic
export default class ApplicationController extends Controller {
  @service config;
  @service system;
  @service token;
  @service notifications;

  /**
   * @type {KeyboardService}
   */
  @service keyboard;

  // eslint-disable-next-line ember/classic-decorator-hooks
  constructor() {
    super(...arguments);
    this.keyboard.listenForKeypress();
  }

  queryParams = [
    {
      region: 'region',
    },
    {
      oneTimeToken: 'ott',
    },
  ];

  region = null;

  oneTimeToken = '';

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

  @computed('error')
  get isOTTExchange() {
    const error = this.error;
    return error instanceof OTTExchangeError;
  }

  @observes('error')
  throwError() {
    if (this.get('config.isDev')) {
      next(() => {
        throw this.error;
      });
    } else if (!Ember.testing) {
      next(() => {
        // eslint-disable-next-line
        console.warn('UNRECOVERABLE ERROR:', this.error);
      });
    }
  }
}
