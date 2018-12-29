import { Factory, faker } from 'ember-cli-mirage';

const DISK_RESERVATIONS = [200, 500, 1000, 2000, 5000, 10000, 100000];

export default Factory.extend({
  name: id => `${faker.hacker.noun().dasherize()}-g-${id}`,
  count: () => faker.random.number({ min: 1, max: 4 }),

  ephemeralDisk: () => ({
    Sticky: faker.random.boolean(),
    SizeMB: faker.random.arrayElement(DISK_RESERVATIONS),
    Migrate: faker.random.boolean(),
  }),

  // Directive used to control whether or not allocations are automatically
  // created.
  createAllocations: true,

  // Directived used to control whether or not the allocation should fail
  // and reschedule, creating reschedule events.
  withRescheduling: false,

  afterCreate(group, server) {
    const tasks = server.createList('task', group.count, {
      taskGroup: group,
    });

    group.update({
      taskIds: tasks.mapBy('id'),
      task_ids: tasks.mapBy('id'),
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
  },
});
