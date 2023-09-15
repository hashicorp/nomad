/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';

export default class AccessControlTokensTokenController extends Controller {
  @service notifications;
  @service router;
  @service store;

  @alias('model.roles') roles;
  @alias('model.token') activeToken; // looks like .token is an Ember reserved name?
  @alias('model.policies') policies;

  @tracked
  error = null;

  @tracked isDeleting = false;

  @action
  onDeletePrompt() {
    this.isDeleting = true;
  }

  @action
  onDeleteCancel() {
    this.isDeleting = false;
  }

  @task(function* () {
    try {
      yield this.activeToken.deleteRecord();
      yield this.activeToken.save();
      this.notifications.add({
        title: 'Token Deleted',
        color: 'success',
        type: `success`,
        destroyOnClick: false,
      });
      this.router.transitionTo('access-control.tokens');
    } catch (err) {
      this.notifications.add({
        title: `Error deleting Token ${this.activeToken.name}`,
        message: err,
        color: 'critical',
        sticky: true,
      });
    }
  })
  deleteToken;
}
