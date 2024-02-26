/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

export default class PolicyRoute extends Route {
  @service store;

  async model(params) {
    return await this.store.findRecord(
      'sentinel-policy',
      decodeURIComponent(params.id),
      {
        reload: true,
      }
    );
  }
}
