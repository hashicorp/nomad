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
  args: () => [],
  command: () => faker.hacker.phrase(),

  afterCreate(action, server) {
    console.log('action has breen created', action, action.task);
    // if (!taskGroupScale.shallow) {
    //   const events = server.createList('scale-event', faker.random.number({ min: 1, max: 10 }));

    //   taskGroupScale.update({
    //     eventIds: events.mapBy('id'),
    //   });
    // }
  },
});
