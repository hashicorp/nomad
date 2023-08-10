/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory, trait } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide, pickOne } from '../utils';
import { DATACENTERS, HOSTS, generateResources } from '../common';
import moment from 'moment';

const UUIDS = provide(100, faker.random.uuid.bind(faker.random));
const NODE_STATUSES = ['initializing', 'ready', 'down'];
const NODE_CLASSES = provide(7, faker.company.bsBuzz.bind(faker.company));
const NODE_VERSIONS = [
  '1.1.0-beta',
  '1.0.2-alpha+ent',
  ...provide(5, faker.system.semver),
];
const REF_DATE = new Date();

export default Factory.extend({
  id: (i) => (i / 100 >= 1 ? `${UUIDS[i]}-${i}` : UUIDS[i]),
  name: (i) => `nomad@${HOSTS[i % HOSTS.length]}`,

  datacenter: () => faker.helpers.randomize(DATACENTERS),
  nodeClass: () => faker.helpers.randomize(NODE_CLASSES),
  drain: faker.random.boolean,
  status: () => faker.helpers.randomize(NODE_STATUSES),
  tlsEnabled: faker.random.boolean,
  schedulingEligibility: () =>
    faker.random.boolean() ? 'eligible' : 'ineligible',

  createIndex: (i) => i,
  modifyIndex: () => faker.random.number({ min: 10, max: 2000 }),
  version: () => faker.helpers.randomize(NODE_VERSIONS),

  httpAddr() {
    return this.name.split('@')[1];
  },

  forceIPv4: trait({
    name: (i) => {
      const ipv4Hosts = HOSTS.filter((h) => !h.startsWith('['));
      return `nomad@${ipv4Hosts[i % ipv4Hosts.length]}`;
    },
  }),

  draining: trait({
    drain: true,
    schedulingEligibility: 'ineligible',
    drainStrategy: {
      Deadline:
        faker.random.number({ min: 30 * 1000, max: 5 * 60 * 60 * 1000 }) *
        1000000,
      ForceDeadline: moment(REF_DATE).add(
        faker.random.number({ min: 1, max: 5 }),
        'd'
      ),
      IgnoreSystemJobs: faker.random.boolean(),
    },
  }),

  forcedDraining: trait({
    drain: true,
    schedulingEligibility: 'ineligible',
    drainStrategy: {
      Deadline: -1,
      ForceDeadline: '0001-01-01T00:00:00Z',
      IgnoreSystemJobs: faker.random.boolean(),
    },
  }),

  noDeadlineDraining: trait({
    drain: true,
    schedulingEligibility: 'ineligible',
    drainStrategy: {
      Deadline: 0,
      ForceDeadline: '0001-01-01T00:00:00Z',
      IgnoreSystemJobs: faker.random.boolean(),
    },
  }),

  noHostVolumes: trait({
    hostVolumes: () => ({}),
  }),

  reserved: trait({
    reservedResources: generateResources({
      CPU: 1000,
      MemoryMB: 2000,
    }),
  }),

  drainStrategy: null,

  drivers: makeDrivers,

  hostVolumes: makeHostVolumes,

  nodeResources: generateResources,

  attributes() {
    // TODO add variability to these
    return {
      'os.version': '10.12.5',
      'cpu.modelname': 'Intel(R) Core(TM) i7-3615QM CPU @ 2.30GHz',
      'nomad.revision': 'f551dcb83e3ac144c9dbb90583b6e82d234662e9',
      'driver.docker.volumes.enabled': '1',
      'driver.docker': '1',
      'cpu.frequency': '2300',
      'memory.totalbytes': '17179869184',
      'driver.mock_driver': '1',
      'kernel.version': '16.6.0',
      'unique.network.ip-address': '127.0.0.1',
      'nomad.version': '0.5.5dev',
      'unique.hostname': 'bacon-mac',
      'cpu.arch': 'amd64',
      'os.name': 'darwin',
      'kernel.name': 'darwin',
      'unique.storage.volume': '/dev/disk1',
      'driver.docker.version': '17.03.1-ce',
      'cpu.totalcompute': '18400',
      'unique.storage.bytestotal': '249783500800',
      'cpu.numcores': '8',
      'os.signals':
        'SIGCONT,SIGSTOP,SIGSYS,SIGINT,SIGIOT,SIGXCPU,SIGSEGV,SIGUSR1,SIGTTIN,SIGURG,SIGUSR2,SIGABRT,SIGALRM,SIGCHLD,SIGFPE,SIGTSTP,SIGIO,SIGKILL,SIGQUIT,SIGXFSZ,SIGBUS,SIGHUP,SIGPIPE,SIGPROF,SIGTRAP,SIGTTOU,SIGILL,SIGTERM',
      'driver.raw_exec': '1',
      'unique.storage.bytesfree': '142954643456',
    };
  },

  withMeta: trait({
    meta: {
      just: 'some',
      prop: 'erties',
      'over.here': 100,
    },
  }),

  afterCreate(node, server) {
    Ember.assert(
      '[Mirage] No node pools! make sure node pools are created before nodes',
      server.db.nodePools.length
    );

    // Each node has a corresponding client stat resource that's queried via node IP.
    // Create that record, even though it's not a relationship.
    server.create('client-stat', {
      id: node.httpAddr,
    });

    const events = server.createList(
      'node-event',
      faker.random.number({ min: 1, max: 10 }),
      { nodeId: node.id }
    );
    const nodePool = node.nodePool
      ? server.db.nodePools.findBy({ name: node.nodePool })
      : pickOne(server.db.nodePools, (pool) => pool.name !== 'all');

    node.update({
      nodePool: nodePool.name,
      eventIds: events.mapBy('id'),
    });

    server.create('client-stat', {
      id: node.id,
    });
  },
});

function makeDrivers() {
  const generate = (name) => {
    const detected = faker.random.number(10) >= 3;
    const healthy = detected && faker.random.number(10) >= 3;
    const attributes = {
      [`driver.${name}.version`]: '1.0.0',
      [`driver.${name}.status`]: 'awesome',
      [`driver.${name}.more.details`]: 'yeah',
      [`driver.${name}.more.again`]: 'we got that',
    };
    return {
      Detected: detected,
      Healthy: healthy,
      HealthDescription: healthy ? 'Driver is healthy' : 'Uh oh',
      UpdateTime: faker.date.past(5 / 365, REF_DATE),
      Attributes: faker.random.number(10) >= 3 && detected ? attributes : null,
    };
  };

  return {
    docker: generate('docker'),
    rkt: generate('rkt'),
    qemu: generate('qemu'),
    exec: generate('exec'),
    raw_exec: generate('raw_exec'),
    java: generate('java'),
  };
}

function makeHostVolumes() {
  const generate = () => ({
    Name: faker.internet.domainWord(),
    Path: `/${faker.internet.userName()}/${faker.internet.domainWord()}/${faker.internet.color()}`,
    ReadOnly: faker.random.boolean(),
  });

  const volumes = provide(faker.random.number({ min: 1, max: 5 }), generate);
  return volumes.reduce((hash, volume) => {
    hash[volume.Name] = volume;
    return hash;
  }, {});
}
