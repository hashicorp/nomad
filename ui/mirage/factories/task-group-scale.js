/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory, trait } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';

export default Factory.extend({
  name() {
    return this.id;
  },

  desired: 1,
  placed: 1,
  running: 1,
  healthy: 1,
  unhealthy: 1,

  shallow: false,

  afterCreate(taskGroupScale, server) {
    if (!taskGroupScale.shallow) {
      const events = server.createList('scale-event', faker.random.number({ min: 1, max: 10 }));

      taskGroupScale.update({
        eventIds: events.mapBy('id'),
      });
    }
  },
});
