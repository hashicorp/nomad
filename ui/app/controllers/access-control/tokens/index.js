/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { task } from 'ember-concurrency';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class AccessControlTokensIndexController extends Controller {
  @service notifications;
  @service router;

  @task(function* (token) {
    try {
      yield token.deleteRecord();
      yield token.save();
      this.notifications.add({
        title: `Token ${token.name} successfully deleted`,
        color: 'success',
      });
    } catch (err) {
      this.error = {
        title: 'Error deleting token',
        description: err,
      };

      throw err;
    }
  })
  deleteToken;

  @action openToken(token) {
    this.router.transitionTo('access-control.tokens.token', token.id);
  }

  @action goToNewToken() {
    this.router.transitionTo('access-control.tokens.new');
  }
}
