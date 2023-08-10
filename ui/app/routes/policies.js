/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default class PoliciesRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service can;
  @service store;
  @service router;

  beforeModel() {
    if (this.can.cannot('list policies')) {
      this.router.transitionTo('/jobs');
    }
  }

  async model() {
    return await hash({
      policies: this.store.query('policy', { reload: true }),
      tokens:
        this.can.can('list tokens') &&
        this.store.query('token', { reload: true }),
    });
  }
}
