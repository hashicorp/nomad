import { Factory, faker } from 'ember-cli-mirage';

export default Factory.extend({
  name: id => `${faker.hacker.noun()}-g-${id}`,
  count: () => faker.random.number({ min: 1, max: 10 }),

  afterCreate(group, server) {
    const tasks = server.createList('task', group.count, {
      taskGroup: group,
    });

    group.update({
      taskIds: tasks.mapBy('id'),
      task_ids: tasks.mapBy('id'),
    });
  },
});
