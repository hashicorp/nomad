/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';

export default class JobsJobServicesServiceRoute extends Route {
  model(params) {
    let { name = '', level = '', tags = '' } = params;
    const services = this.modelFor('jobs.job')
      .get('services')
      .filter(
        (service) =>
          service.name === name &&
          service.derivedLevel === level &&
          // Tags are an array, but queryparam is a string of values separated by commas.
          (tags
            ? tags.split(',').every((tag) => service.tags.includes(tag))
            : true)
      );
    return { name, instances: services || [] };
  }
}
