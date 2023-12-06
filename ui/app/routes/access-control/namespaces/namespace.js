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

export default class AccessControlNamespacesRoleRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service store;

  async model(params) {
    let namespace = await this.store.findRecord(
      'namespace',
      decodeURIComponent(params.name),
      {
        reload: true,
      }
    );

    return hash({
      namespace,
    });
  }
}
