/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default class AccessControlTokensTokenRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service store;

  async model(params) {
    let token = await this.store.findRecord(
      'token',
      decodeURIComponent(params.id),
      {
        reload: true,
      }
    );

    let policies = this.store.peekAll('policy');
    let roles = this.store.peekAll('role');

    return hash({
      token,
      roles,
      policies,
    });
  }
}
