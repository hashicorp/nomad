/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import withForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default class PoliciesPolicyRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service store;
  async model(params) {
    return hash({
      policy: this.store.findRecord('policy', decodeURIComponent(params.name), {
        reload: true,
      }),
      tokens: this.store
        .peekAll('token')
        .filter((token) =>
          token.policyNames?.includes(decodeURIComponent(params.name))
        ),
    });
  }
}
