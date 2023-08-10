/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default class JobsRunTemplatesTemplateRoute extends Route {
  @service can;
  @service router;
  @service store;
  @service system;

  beforeModel(transition) {
    if (
      this.can.cannot('write variable', null, {
        namespace: transition.to.queryParams.namespace,
      })
    ) {
      this.router.transitionTo('jobs.run');
    }
  }

  async model({ name }) {
    try {
      return this.store.findRecord('variable', name);
    } catch (e) {
      notifyForbidden(this)(e);
    }
  }
}
