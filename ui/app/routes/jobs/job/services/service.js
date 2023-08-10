/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';

export default class JobsJobServicesServiceRoute extends Route {
  model({ name = '', level = '' }) {
    const services = this.modelFor('jobs.job')
      .get('services')
      .filter(
        (service) => service.name === name && service.derivedLevel === level
      );
    return { name, instances: services || [] };
  }
}
