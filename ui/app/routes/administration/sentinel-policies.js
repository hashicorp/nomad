/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import classic from 'ember-classic-decorator';

@classic
export default class AdministrationSentinelPoliciesRoute extends Route {
  @service store;

  model() {
    return this.store.findAll('sentinel-policy', { reload: true });
  }
}
