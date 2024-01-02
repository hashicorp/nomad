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
  @service flashMessages;
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
      this.flashMessages.add({
        title: 'Policy Deleted',
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });
      this.router.transitionTo('policies');
    } catch (err) {
      this.flashMessages.add({
        title: `Error deleting Policy ${this.policy.name}`,
        message: err,
        type: 'error',
        destroyOnClick: false,
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
      this.flashMessages.add({
        title: 'Example Token Created',
        message: `${newToken.secret}`,
        type: 'success',
        destroyOnClick: false,
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
      this.flashMessages.add({
        title: 'Token successfully deleted',
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
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
