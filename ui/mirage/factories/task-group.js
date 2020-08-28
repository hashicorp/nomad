import { Factory, trait } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide } from '../utils';

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
      const tasks = provide(group.count, () => {
        const mounts = faker.helpers
          .shuffle(volumes)
          .slice(0, faker.random.number({ min: 1, max: 3 }));
        return server.create('task', {
          taskGroup: group,
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
