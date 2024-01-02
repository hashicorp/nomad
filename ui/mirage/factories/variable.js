/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide, pickOne } from '../utils';

export default Factory.extend({
  id: () => faker.random.words(3).split(' ').join('/').toLowerCase(),
  path() {
    return this.id;
  },
  namespace: null,
  createdIndex: 100,
  modifiedIndex: 100,
  createTime: () => faker.date.past(15) * 1000000,
  modifyTime: () => faker.date.recent(1) * 1000000,
  items() {
    return (
      this.Items || {
        [faker.database.column()]: faker.database.collation(),
        [faker.database.column()]: faker.database.collation(),
        [faker.database.column()]: faker.database.collation(),
        [faker.database.column()]: faker.database.collation(),
        [faker.database.column()]: faker.database.collation(),
      }
    );
  },

  afterCreate(variable, server) {
    if (!variable.namespace) {
      const namespace = pickOne(server.db.jobs)?.namespace || 'default';
      variable.update({
        namespace,
      });
    }
  },
});
