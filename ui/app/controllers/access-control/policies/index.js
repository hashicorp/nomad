/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { task } from 'ember-concurrency';

export default class AccessControlPoliciesIndexController extends Controller {
  @service router;
  @service notifications;

  get policies() {
    return this.model.policies.map((policy) => {
      policy.tokens = (this.model.tokens || []).filter((token) => {
        return token.policies.includes(policy);
      });
      return policy;
    });
  }

  @action openPolicy(policy) {
    this.router.transitionTo('access-control.policies.policy', policy.name);
  }

  @action goToNewPolicy() {
    this.router.transitionTo('access-control.policies.new');
  }

  @task(function* (policy) {
    try {
      yield policy.deleteRecord();
      yield policy.save();
      this.notifications.add({
        title: `Policy ${policy.name} successfully deleted`,
        color: 'success',
      });
    } catch (err) {
      this.error = {
        title: 'Error deleting policy',
        description: err,
      };

      throw err;
    }
  })
  deletePolicy;
}
