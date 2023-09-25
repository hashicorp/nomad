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
}
