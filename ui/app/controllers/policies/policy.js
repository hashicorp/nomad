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

export default class PoliciesPolicyController extends Controller {
  @service notifications;
  @service router;
  @service store;

  @alias('model.policy') policy;
  @alias('model.tokens') tokens;

  @tracked
  error = null;

  @tracked isDeleting = false;

  get newTokenString() {
    return `nomad acl token create -name="<TOKEN_NAME>" -policy="${this.policy.name}" -type=client -ttl=<8h>`;
  }

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
      yield this.policy.deleteRecord();
      yield this.policy.save();
      this.notifications.add({
        title: 'Policy Deleted',
        color: 'success',
        type: `success`,
        destroyOnClick: false,
      });
      this.router.transitionTo('policies');
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
      this.error = {
        title: 'Error creating new token',
        description: err,
      };

      throw err;
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
      this.error = {
        title: 'Error deleting token',
        description: err,
      };

      throw err;
    }
  })
  deleteToken;
}
