/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class NewRoute extends Route {
  @service store;

  model() {
    return this.store.createRecord('sentinel-policy', {
      name: '',
      policy: 'main = rule { true }\n',
      enforcementLevel: 'advisory',
      scope: 'submit-job',
    });
  }
}
