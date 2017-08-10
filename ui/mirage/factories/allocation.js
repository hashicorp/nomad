import Ember from 'ember';
import { Factory, faker, trait } from 'ember-cli-mirage';
import { provide, pickOne } from '../utils';

const UUIDS = provide(100, faker.random.uuid.bind(faker.random));

export default Factory.extend({
  id: i => (i / 100 >= 1 ? `${UUIDS[i]}-${i}` : UUIDS[i]),

  withTaskWithPorts: trait({
    afterCreate(allocation, server) {
      const taskGroup = server.db.taskGroups.findBy({ name: allocation.taskGroup });
      const resources = taskGroup.taskIds.map(id =>
        server.create(
          'task-resources',
          {
            allocation,
            name: server.db.tasks.find(id).name,
          },
          'withReservedPorts'
        )
      );

      allocation.update({ taskResourcesIds: resources.mapBy('id') });
    },
  }),

  afterCreate(allocation, server) {
    Ember.assert(
      '[Mirage] No jobs! make sure jobs are created before allocations',
      server.db.jobs.length
    );
    Ember.assert(
      '[Mirage] No nodes! make sure nodes are created before allocations',
      server.db.nodes.length
    );

    const job = allocation.job || pickOne(server.db.jobs);
    const node = allocation.node || pickOne(server.db.nodes);
    const taskGroup = allocation.taskGroup
      ? server.db.taskGroups.findBy({ name: allocation.taskGroup })
      : pickOne(server.db.taskGroups.where({ jobId: job.id }));

    const states = taskGroup.taskIds.map(id =>
      server.create('task-state', {
        allocation,
        name: server.db.tasks.find(id).name,
      })
    );

    const resources = taskGroup.taskIds.map(id =>
      server.create('task-resources', {
        allocation,
        name: server.db.tasks.find(id).name,
      })
    );

    allocation.update({
      jobId: job.id,
      nodeId: node.id,
      taskStateIds: states.mapBy('id'),
      task_state_ids: states.mapBy('id'),
      taskResourcesIds: resources.mapBy('id'),
      taskGroup: taskGroup.name,
      name: `${taskGroup.name}.[${faker.random.number(10)}]`,
    });

    // Each allocation has a corresponding allocation stats running on some client.
    // Create that record, even though it's not a relationship.
    server.create('client-allocation-stats', {
      id: allocation.id,
      _tasks: states.mapBy('name'),
    });
  },
});
