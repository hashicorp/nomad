/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory, trait } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';
import { DATACENTERS } from '../common';
import { scenario } from '../scenarios/default';

const UUIDS = provide(100, faker.random.uuid.bind(faker.random));
const AGENT_STATUSES = ['alive', 'leaving', 'left', 'failed'];
const AGENT_BUILDS = [
  '1.1.0-beta',
  '1.0.2-alpha+ent',
  ...provide(5, faker.system.semver),
];

export default Factory.extend({
  id: (i) => (i / 100 >= 1 ? `${UUIDS[i]}-${i}` : UUIDS[i]),

  name: () => generateName(),

  config: {
    UI: {
      Enabled: true,
      Label: {
        TextColor: 'white',
        BackgroundColor: 'hotpink',
        Text: `Mirage - ${scenario}`,
      },
    },
    ACL: {
      Enabled: true,
    },
    Version: {
      Version: '1.1.0',
      VersionMetadata: 'ent',
      VersionPrerelease: 'dev',
    },
  },

  member() {
    const serfPort = faker.random.number({ min: 4000, max: 4999 });
    return {
      Name: this.name,
      Port: serfPort,
      Status: faker.helpers.randomize(AGENT_STATUSES),
      Address: generateAddress(this.name),
      Tags: generateTags(serfPort),
    };
  },

  version() {
    return this.member.Tags?.build || '';
  },

  withConsulLink: trait({
    afterCreate(agent) {
      agent.config.UI.Consul = {
        BaseUIURL: 'http://localhost:8500/ui',
      };
    },
  }),

  withVaultLink: trait({
    afterCreate(agent) {
      agent.config.UI.Vault = {
        BaseUIURL: 'http://localhost:8200/ui',
      };
    },
  }),
});

function generateName() {
  return `nomad@${
    faker.random.boolean() ? faker.internet.ip() : faker.internet.ipv6()
  }`;
}

function generateAddress(name) {
  return name.split('@')[1];
}

function generateTags(serfPort) {
  const rpcPortCandidate = faker.random.number({ min: 4000, max: 4999 });
  return {
    port:
      rpcPortCandidate === serfPort ? rpcPortCandidate + 1 : rpcPortCandidate,
    dc: faker.helpers.randomize(DATACENTERS),
    build: faker.helpers.randomize(AGENT_BUILDS),
  };
}
