/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';

export default class AccessControlRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service can;
  @service store;
  @service router;

  beforeModel() {
    // TODO: policy listing is currently selfTokenIsManagement dependent.
    // If this should ever change and become its own ACL permission, we should
    // expand this to include roles and tokens checks as well. Otherwise, they're
    // all the same permission.
    if (this.can.cannot('list policies')) {
      this.router.transitionTo('/jobs');
    }
  }

  // Load our tokens, roles, and policies
  async model() {
    return {
      tokens: await this.store.findAll('token'),
      roles: await this.store.findAll('role'),
      policies: await this.store.findAll('policy'),
    };
  }
}
