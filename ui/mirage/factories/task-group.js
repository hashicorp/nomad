import { Factory, trait } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';
import { generateResources } from '../common';

const DISK_RESERVATIONS = [200, 500, 1000, 2000, 5000, 10000, 100000];

export default Factory.extend({
  name: id => `${faker.hacker.noun().dasherize()}-g-${id}`,
  count: () => faker.random.number({ min: 1, max: 2 }),

  ephemeralDisk: () => ({
    Sticky: faker.random.boolean(),
    SizeMB: faker.helpers.randomize(DISK_RESERVATIONS),
    Migrate: faker.random.boolean(),
  }),

  noHostVolumes: trait({
    volumes: () => ({}),
  }),

  withScaling: faker.random.boolean,

  volumes: makeHostVolumes(),

  // Directive used to control whether or not allocations are automatically
  // created.
  createAllocations: true,

  // Directived used to control whether or not the allocation should fail
  // and reschedule, creating reschedule events.
  withRescheduling: false,

  // Directive used to control whether the task group should have services.
  withServices: false,

  // When true, only creates allocations
  shallow: false,

  // When set, passed into tasks to set resource values
  resourceSpec: null,

  afterCreate(group, server) {
    let taskIds = [];
    let volumes = Object.keys(group.volumes);

    if (group.withScaling) {
      group.update({
        scaling: {
          Min: 1,
          Max: 5,
          Policy: faker.random.boolean() && {
            EvaluationInterval: '10s',
            Cooldown: '2m',
            Check: {
              avg_conn: {
                Source: 'prometheus',
                Query:
                  'scalar(avg((haproxy_server_current_sessions{backend="http_back"}) and (haproxy_server_up{backend="http_back"} == 1)))',
                Strategy: {
                  'target-value': {
                    target: 20,
                  },
                },
              },
            },
          },
        },
      });
    }

    if (!group.shallow) {
      const resources =
        group.resourceSpec && divide(group.count, parseResourceSpec(group.resourceSpec));
      const tasks = provide(group.count, (_, idx) => {
        const mounts = faker.helpers
          .shuffle(volumes)
          .slice(0, faker.random.number({ min: 1, max: 3 }));

        const maybeResources = {};
        if (resources) {
          maybeResources.Resources = generateResources(resources[idx]);
        }
        return server.create('task', {
          taskGroup: group,
          ...maybeResources,
          volumeMounts: mounts.map(mount => ({
            Volume: mount,
            Destination: `/${faker.internet.userName()}/${faker.internet.domainWord()}/${faker.internet.color()}`,
            PropagationMode: '',
            ReadOnly: faker.random.boolean(),
          })),
        });
      });
      taskIds = tasks.mapBy('id');
    }

    group.update({
      taskIds: taskIds,
    });

    if (group.createAllocations) {
      Array(group.count)
        .fill(null)
        .forEach((_, i) => {
          const props = {
            jobId: group.job.id,
            namespace: group.job.namespace,
            taskGroup: group.name,
            name: `${group.name}.[${i}]`,
            rescheduleSuccess: group.withRescheduling ? faker.random.boolean() : null,
            rescheduleAttempts: group.withRescheduling
              ? faker.random.number({ min: 1, max: 5 })
              : 0,
          };

          if (group.withRescheduling) {
            server.create('allocation', 'rescheduled', props);
          } else {
            server.create('allocation', props);
          }
        });
    }

    if (group.withServices) {
      Array(faker.random.number({ min: 1, max: 3 }))
        .fill(null)
        .forEach(() => {
          server.create('service', {
            taskGroup: group,
          });
        });
    }
  },
});

function makeHostVolumes() {
  const generate = () => ({
    Name: faker.internet.domainWord(),
    Type: 'host',
    Source: faker.internet.domainWord(),
    ReadOnly: faker.random.boolean(),
  });

  const volumes = provide(faker.random.number({ min: 1, max: 5 }), generate);
  return volumes.reduce((hash, volume) => {
    hash[volume.Name] = volume;
    return hash;
  }, {});
}

function parseResourceSpec(spec) {
  const mapping = {
    M: 'MemoryMB',
    C: 'CPU',
    D: 'DiskMB',
    I: 'IOPS',
  };

  const terms = spec.split(',').map(t => {
    const [k, v] = t
      .trim()
      .split(':')
      .map(kv => kv.trim());
    return [k, +v];
  });

  return terms.reduce((hash, term) => {
    hash[mapping[term[0]]] = term[1];
    return hash;
  }, {});
}

// Split a single resources object into N resource objects where
// the sum of each property of the new resources objects equals
// the original resources properties
// ex: divide(2, { Mem: 400, Cpu: 250 }) -> [{ Mem: 80, Cpu: 50 }, { Mem: 320, Cpu: 200 }]
function divide(count, resources) {
  const wheel = roulette(1, count);

  const ret = provide(count, (_, idx) => {
    return Object.keys(resources).reduce((hash, key) => {
      hash[key] = Math.round(resources[key] * wheel[idx]);
      return hash;
    }, {});
  });

  return ret;
}

// Roulette splits a number into N divisions
// Variance is a value between 0 and 1 that determines how much each division in
// size. At 0 each division is even, at 1, it's entirely random but the sum of all
// divisions is guaranteed to equal the total value.
function roulette(total, divisions, variance = 0.8) {
  let roulette = new Array(divisions).fill(total / divisions);
  roulette.forEach((v, i) => {
    if (i === roulette.length - 1) return;
    roulette.splice(i, 2, ...rngDistribute(roulette[i], roulette[i + 1], variance));
  });
  return roulette;
}

function rngDistribute(a, b, variance = 0.8) {
  const move = a * faker.random.number({ min: 0, max: variance, precision: 0.01 });
  return [a - move, b + move];
}
