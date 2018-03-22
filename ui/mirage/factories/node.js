import { Factory, faker, trait } from 'ember-cli-mirage';
import { provide } from '../utils';
import { DATACENTERS, HOSTS } from '../common';

const UUIDS = provide(100, faker.random.uuid.bind(faker.random));
const NODE_STATUSES = ['initializing', 'ready', 'down'];

export default Factory.extend({
  id: i => (i / 100 >= 1 ? `${UUIDS[i]}-${i}` : UUIDS[i]),
  name: i => `nomad@${HOSTS[i % HOSTS.length]}`,

  datacenter: faker.list.random(...DATACENTERS),
  isDraining: faker.random.boolean,
  status: faker.list.random(...NODE_STATUSES),
  tls_enabled: faker.random.boolean,

  createIndex: i => i,
  modifyIndex: () => faker.random.number({ min: 10, max: 2000 }),

  httpAddr() {
    return this.name.split('@')[1];
  },

  forceIPv4: trait({
    name: i => {
      const ipv4Hosts = HOSTS.filter(h => !h.startsWith('['));
      return `nomad@${ipv4Hosts[i % ipv4Hosts.length]}`;
    },
  }),

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
    // Each node has a corresponding client stats resource that's queried via node IP.
    // Create that record, even though it's not a relationship.
    server.create('client-stats', {
      id: node.httpAddr,
    });
  },
});
