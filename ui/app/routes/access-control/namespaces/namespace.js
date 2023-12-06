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

export default class AccessControlNamespacesHamespaceRoute extends Route.extend(
  withForbiddenState,
  WithModelErrorHandling
) {
  @service store;

  async model(params) {
    console.log('model hit');
    let namespace = await this.store.findRecord(
      'namespace',
      decodeURIComponent(params.name),
      {
        reload: true,
      }
    );

    console.log('ns', namespace);

    return hash({
      namespace,
    });
  }
}
