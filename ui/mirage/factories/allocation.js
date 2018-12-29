import Ember from 'ember';
import moment from 'moment';
import { Factory, faker, trait } from 'ember-cli-mirage';
import { provide, pickOne } from '../utils';

const UUIDS = provide(100, faker.random.uuid.bind(faker.random));
const CLIENT_STATUSES = ['pending', 'running', 'complete', 'failed', 'lost'];
const DESIRED_STATUSES = ['run', 'stop', 'evict'];
const REF_TIME = new Date();

export default Factory.extend({
  id: i => (i >= 100 ? `${UUIDS[i % 100]}-${i}` : UUIDS[i]),

  jobVersion: () => faker.random.number(10),

  modifyIndex: () => faker.random.number({ min: 10, max: 2000 }),
  modifyTime: () => faker.date.past(2 / 365, REF_TIME) * 1000000,

  createIndex: () => faker.random.number({ min: 10, max: 2000 }),
  createTime() {
    return faker.date.past(2 / 365, new Date(this.modifyTime / 1000000)) * 1000000;
  },

  namespace: null,

  clientStatus: faker.list.random(...CLIENT_STATUSES),
  desiredStatus: faker.list.random(...DESIRED_STATUSES),

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

  withoutTaskWithPorts: trait({
    afterCreate(allocation, server) {
      const taskGroup = server.db.taskGroups.findBy({ name: allocation.taskGroup });
      const resources = taskGroup.taskIds.map(id =>
        server.create(
          'task-resources',
          {
            allocation,
            name: server.db.tasks.find(id).name,
          },
          'withoutReservedPorts'
        )
      );

      allocation.update({ taskResourcesIds: resources.mapBy('id') });
    },
  }),

  rescheduleAttempts: 0,
  rescheduleSuccess: false,

  rescheduled: trait({
    // Create another allocation carrying the events of this as well as the reschduleSuccess state.
    // Pass along rescheduleAttempts after decrementing.
    // After rescheduleAttempts hits zero, a final allocation is made with no nextAllocation and
    // a clientStatus of failed or running, depending on rescheduleSuccess
    afterCreate(allocation, server) {
      const attempts = allocation.rescheduleAttempts - 1;
      const previousEvents =
        (allocation.rescheduleTracker && allocation.rescheduleTracker.Events) || [];

      let rescheduleTime;
      if (previousEvents.length) {
        const lastEvent = previousEvents[previousEvents.length - 1];
        rescheduleTime = moment(lastEvent.RescheduleTime / 1000000).add(5, 'minutes');
      } else {
        rescheduleTime = faker.date.past(2 / 365, REF_TIME);
      }

      rescheduleTime *= 1000000;

      const rescheduleTracker = {
        Events: previousEvents.concat([
          {
            PrevAllocID: allocation.id,
            PrevNodeID: null, //allocation.node.id,
            RescheduleTime: rescheduleTime,
          },
        ]),
      };

      let nextAllocation;
      if (attempts > 0) {
        nextAllocation = server.create('allocation', 'rescheduled', {
          rescheduleAttempts: Math.max(attempts, 0),
          rescheduleSuccess: allocation.rescheduleSuccess,
          previousAllocation: allocation.id,
          clientStatus: 'failed',
          rescheduleTracker,
          followupEvalId: server.create('evaluation', {
            waitUntil: rescheduleTime,
          }).id,
        });
      } else {
        nextAllocation = server.create('allocation', {
          previousAllocation: allocation.id,
          clientStatus: allocation.rescheduleSuccess ? 'running' : 'failed',
          rescheduleTracker,
        });
      }

      allocation.update({ nextAllocation: nextAllocation.id, clientStatus: 'failed' });
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

    const job = allocation.jobId ? server.db.jobs.find(allocation.jobId) : pickOne(server.db.jobs);
    const namespace = allocation.namespace || job.namespace;
    const node = allocation.nodeId
      ? server.db.nodes.find(allocation.nodeId)
      : pickOne(server.db.nodes);
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
      namespace,
      jobId: job.id,
      nodeId: node.id,
      taskStateIds: allocation.clientStatus === 'pending' ? [] : states.mapBy('id'),
      taskResourcesIds: allocation.clientStatus === 'pending' ? [] : resources.mapBy('id'),
      taskGroup: taskGroup.name,
      name: allocation.name || `${taskGroup.name}.[${faker.random.number(10)}]`,
    });

    // Each allocation has a corresponding allocation stats running on some client.
    // Create that record, even though it's not a relationship.
    server.create('client-allocation-stats', {
      id: allocation.id,
      _tasks: states.mapBy('name'),
    });
  },
});
