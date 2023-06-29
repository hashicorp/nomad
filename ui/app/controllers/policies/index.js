/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class PoliciesIndexController extends Controller {
  @service router;
  get policies() {
    return this.model.policies.map((policy) => {
      policy.tokens = (this.model.tokens || []).filter((token) => {
        return token.policies.includes(policy);
      });
      return policy;
    });
  }

  get roles() {
    return this.model.roles.map((role) => {
      role.tokens = (this.model.tokens || []).filter((token) => {
        console.log(
          'tokin',
          token,
          token.roles,
          token.roles.length,
          token.policies.length
        );
        return token.roles.mapBy('id').includes(role.id);
      });
      return role;
    });
  }

  @action openPolicy(policy) {
    this.router.transitionTo('policies.policy', policy.name);
  }
}
