/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { service } from '@ember/service';
import { task } from 'ember-concurrency';

export default class AccessControlTokensTokenController extends Controller {
  @service notifications;
  @service router;
  @service store;

  get roles() {
    return this.model.roles;
  }

  get activeToken() {
    return this.model.token;
  }

  get policies() {
    return this.model.policies;
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
      this.router.transitionTo('administration.tokens');
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
