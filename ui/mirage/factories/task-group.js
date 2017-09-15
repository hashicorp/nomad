import { Factory, faker } from 'ember-cli-mirage';

export default Factory.extend({
  name: id => `${faker.hacker.noun()}-g-${id}`,
  count: () => faker.random.number({ min: 1, max: 4 }),

  // Directive used to control whether or not allocations are automatically
  // created.
  createAllocations: true,

  afterCreate(group, server) {
    const tasks = server.createList('task', group.count, {
      taskGroup: group,
    });

    group.update({
      taskIds: tasks.mapBy('id'),
      task_ids: tasks.mapBy('id'),
    });

    if (group.createAllocations) {
      Array(group.count).fill(null).forEach((_, i) => {
        server.create('allocation', {
          jobId: group.job.id,
          taskGroup: group.name,
          name: `${group.name}.[${i}]`,
        });
      });
    }
  },
});
