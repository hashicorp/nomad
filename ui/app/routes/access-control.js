/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';
import RSVP from 'rsvp';

export default class AccessControlRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service can;
  @service store;
  @service router;

  beforeModel() {
    if (
      this.can.cannot('list policies') ||
      this.can.cannot('list roles') ||
      this.can.cannot('list tokens')
    ) {
      this.router.transitionTo('/jobs');
    }
  }

  // Load our tokens, roles, and policies
  model() {
    console;
    return RSVP.hash({
      tokens: this.store.findAll('token'),
      roles: this.store.findAll('role'),
      policies: this.store.findAll('policy'),
    });
  }

  // After model: check for all tokens[].policies and roles[].policies to see if any of them are listed
  // that aren't also in the policies list.
  // If any of them are, unload them from the store — they are orphans.
  afterModel(model) {
    let policies = model.policies;
    let roles = model.roles;
    let tokens = model.tokens;

    roles.forEach((role) => {
      let orphanedPolicies = [];
      role.policies.forEach((policy) => {
        if (policy && !policies.includes(policy)) {
          orphanedPolicies.push(policy);
        }
      });
      orphanedPolicies.forEach((policy) => {
        role.policies.removeObject(policy);
        this.store.unloadRecord(policy);
      });
    });

    tokens.forEach((token) => {
      let orphanedPolicies = [];
      token.policies.forEach((policy) => {
        if (policy && !policies.includes(policy)) {
          orphanedPolicies.push(policy);
        }
      });
      orphanedPolicies.forEach((policy) => {
        token.policies.removeObject(policy);
        this.store.unloadRecord(policy);
      });
    });
  }
}
