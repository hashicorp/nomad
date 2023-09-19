/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class TokenEditorComponent extends Component {
  @service notifications;
  @service router;
  @service store;

  @alias('args.roles') roles;
  @alias('args.token') activeToken;
  @alias('args.policies') policies;

  @tracked tokenPolicies = [];
  @tracked tokenRoles = [];

  // when this renders, set up tokenPolicies
  constructor() {
    super(...arguments);
    this.tokenPolicies = this.activeToken.policies.toArray() || [];
    this.tokenRoles = this.activeToken.roles.toArray() || [];
    if (this.activeToken.isNew) {
      this.activeToken.expirationTTL = 'never';
    }
  }

  @action updateTokenPolicies(policy, event) {
    let { checked } = event.target;
    if (checked) {
      this.tokenPolicies.push(policy);
    } else {
      this.tokenPolicies = this.tokenPolicies.filter((p) => p !== policy);
    }
  }

  @action updateTokenRoles(role, event) {
    let { checked } = event.target;
    if (checked) {
      this.tokenRoles.push(role);
    } else {
      this.tokenRoles = this.tokenRoles.filter((p) => p !== role);
    }
  }

  @action updateTokenType(event) {
    let tokenType = event.target.id;
    this.activeToken.type = tokenType;
  }

  @action updateTokenExpirationTime(event) {
    // Override expirationTTL if user selects a time
    this.activeToken.expirationTTL = null;
    this.activeToken.expirationTime = new Date(event.target.value);
  }
  @action updateTokenExpirationTTL(event) {
    // Override expirationTime if user selects a TTL
    this.activeToken.expirationTime = null;
    if (event.target.value === 'never') {
      this.activeToken.expirationTTL = null;
    } else if (event.target.value === 'custom') {
      this.activeToken.expirationTime = new Date();
    } else {
      this.activeToken.expirationTTL = event.target.value;
    }
  }

  @action async save() {
    try {
      const shouldRedirectAfterSave = this.activeToken.isNew;

      this.activeToken.policies = this.tokenPolicies;
      this.activeToken.roles = this.tokenRoles;

      if (this.activeToken.type === 'management') {
        // Management tokens cannot have policies or roles
        this.activeToken.policyIDs = [];
        this.activeToken.policyNames = [];
        this.activeToken.policies = [];
        this.activeToken.roles = [];
      }

      await this.activeToken.save();

      this.notifications.add({
        title: 'Token Saved',
        color: 'success',
      });

      if (shouldRedirectAfterSave) {
        this.router.transitionTo(
          'access-control.tokens.token',
          this.activeToken.id
        );
      }
    } catch (error) {
      this.notifications.add({
        title: `Error creating Token ${this.activeToken.name}`,
        message: error,
        color: 'critical',
        sticky: true,
      });
    }
  }
}
