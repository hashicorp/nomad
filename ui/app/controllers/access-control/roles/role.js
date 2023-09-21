/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';

export default class AccessControlRolesRoleController extends Controller {
  @service notifications;
  @service router;
  @service store;

  @alias('model.role') role;
  @alias('model.tokens') tokens;
  @alias('model.policies') policies;

  get newTokenString() {
    return `nomad acl token create -name="<TOKEN_NAME>" -role-name="${this.role.name}" -type=client -ttl=8h`;
  }

  @task(function* () {
    try {
      yield this.role.deleteRecord();
      yield this.role.save();
      this.notifications.add({
        title: 'Role Deleted',
        color: 'success',
        type: `success`,
        destroyOnClick: false,
      });
      this.router.transitionTo('access-control.roles');
    } catch (err) {
      this.notifications.add({
        title: `Error deleting Role ${this.role.name}`,
        message: err,
        color: 'critical',
        sticky: true,
      });
    }
  })
  deleteRole;

  async refreshTokens() {
    this.tokens = this.store.peekAll('token').filter((token) =>
      token.roles.any((role) => {
        return role.id === decodeURIComponent(this.role.id);
      })
    );
  }

  @task(function* () {
    try {
      const newToken = this.astore.createRecord('token', {
        name: `Example Token for ${this.role.name}`,
        roles: [this.role],
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
