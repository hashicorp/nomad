/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';

export default class AccessControlPoliciesPolicyController extends Controller {
  @service notifications;
  @service router;
  @service store;

  @alias('model.policy') policy;
  @alias('model.tokens') tokens;

  get newTokenString() {
    return `nomad acl token create -name="<TOKEN_NAME>" -policy="${this.policy.name}" -type=client -ttl=8h`;
  }
  @task(function* () {
    try {
      yield this.policy.deleteRecord();
      yield this.policy.save();

      // Cleanup: Remove references from roles and tokens
      this.store.peekAll('role').forEach((role) => {
        role.policies.removeObject(this.policy);
      });
      this.store.peekAll('token').forEach((token) => {
        token.policies.removeObject(this.policy);
      });
      if (this.store.peekRecord('policy', this.policy.id)) {
        this.store.unloadRecord(this.policy);
      }

      this.notifications.add({
        title: 'Policy Deleted',
        color: 'success',
        type: `success`,
        destroyOnClick: false,
      });
      this.router.transitionTo('access-control.policies');
    } catch (err) {
      this.notifications.add({
        title: `Error deleting Policy ${this.policy.name}`,
        message: err,
        color: 'critical',
        sticky: true,
      });
    }
  })
  deletePolicy;

  async refreshTokens() {
    this.tokens = this.store
      .peekAll('token')
      .filter((token) =>
        token.policyNames?.includes(decodeURIComponent(this.policy.name))
      );
  }

  @task(function* () {
    try {
      const newToken = this.store.createRecord('token', {
        name: `Example Token for ${this.policy.name}`,
        policies: [this.policy],
        // New date 10 minutes into the future
        expirationTime: new Date(Date.now() + 10 * 60 * 1000),
        type: 'client',
      });
      yield newToken.save();
      yield this.refreshTokens();
      this.notifications.add({
        title: 'Example Token Created',
        message: `${newToken.secret}`,
        color: 'success',
        timeout: 30000,
        customAction: {
          label: 'Copy to Clipboard',
          action: () => {
            navigator.clipboard.writeText(newToken.secret);
          },
        },
      });
    } catch (err) {
      this.notifications.add({
        title: 'Error creating test token',
        message: err,
        color: 'critical',
        sticky: true,
      });
    }
  })
  createTestToken;

  @task(function* (token) {
    try {
      yield token.deleteRecord();
      yield token.save();
      yield this.refreshTokens();
      this.notifications.add({
        title: 'Token successfully deleted',
        color: 'success',
      });
    } catch (err) {
      this.notifications.add({
        title: 'Error deleting token',
        message: err,
        color: 'critical',
        sticky: true,
      });
    }
  })
  deleteToken;
}
