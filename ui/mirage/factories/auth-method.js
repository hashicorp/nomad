/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide, pickOne } from '../utils';

export default Factory.extend({
  name: () => pickOne(['vault', 'auth0', 'github', 'cognito', 'okta']),
  type: () => pickOne(['kubernetes', 'jwt', 'oidc', 'ldap', 'radius']),
  tokenLocality: () => pickOne(['local', 'global']),
  maxTokenTTL: () => faker.random.number({ min: 1, max: 1000 }) + 'h',
  default: () => faker.random.boolean(),
  createTime: () => faker.date.past(),
  createIndex: () => faker.random.number(),
  modifyTime: () => faker.date.past(),
  modifyIndex: () => faker.random.number(),
});
