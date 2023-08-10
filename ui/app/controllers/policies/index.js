/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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

  @action openPolicy(policy) {
    this.router.transitionTo('policies.policy', policy.name);
  }
}
