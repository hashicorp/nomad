/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class JobServiceRowComponent extends Component {
  @service router;
  @service system;

  @action
  gotoService(service) {
    if (service.provider === 'nomad') {
      this.router.transitionTo('jobs.job.services.service', service.name, {
        queryParams: { level: service.level },
        instances: service.instances,
      });
    }
  }

  get consulRedirectLink() {
    return this.system.agent.get('config')?.UI?.Consul?.BaseUIURL;
  }
}
