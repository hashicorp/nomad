/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Ember from 'ember';
import moment from 'moment';
import { Factory, trait } from 'ember-cli-mirage';
import faker from 'nomad-ui/mirage/faker';
import { provide, pickOne } from '../utils';
import { generateResources } from '../common';

const UUIDS = provide(100, faker.random.uuid.bind(faker.random));
const CLIENT_STATUSES = ['pending', 'running', 'complete', 'failed', 'lost'];
const DESIRED_STATUSES = ['run', 'stop', 'evict'];
const REF_TIME = new Date();

export default Factory.extend({
  id: (i) => (i >= 100 ? `${UUIDS[i % 100]}-${i}` : UUIDS[i]),

  jobVersion: 1,

  modifyIndex: () => faker.random.number({ min: 10, max: 2000 }),
  modifyTime: () => faker.date.past(2 / 365, REF_TIME) * 1000000,

  createIndex: () => faker.random.number({ min: 10, max: 2000 }),
  createTime() {
    return (
      faker.date.past(2 / 365, new Date(this.modifyTime / 1000000)) * 1000000
    );
  },

  namespace: null,

  clientStatus() {
    return this.forceRunningClientStatus
      ? 'running'
      : faker.helpers.randomize(CLIENT_STATUSES);
  },

  desiredStatus: () => faker.helpers.randomize(DESIRED_STATUSES),

  // When true, doesn't create any resources, state, or events
  shallow: false,

  // When true, sets the client status to running
  forceRunningClientStatus: false,

  withTaskWithPorts: trait({
    afterCreate(allocation, server) {
      const taskGroup = server.db.taskGroups.findBy({
        name: allocation.taskGroup,
      });
      const resources = taskGroup.taskIds.map((id) => {
        const task = server.db.tasks.find(id);
        return server.create('task-resource', {
          allocation,
          name: task.name,
          resources: generateResources({
            CPU: task.resources.CPU,
            MemoryMB: task.resources.MemoryMB,
            DiskMB: task.resources.DiskMB,
            networks: { minPorts: 1 },
          }),
        });
      });

      allocation.update({ taskResourceIds: resources.mapBy('id') });
    },
  }),

  withoutTaskWithPorts: trait({
    afterCreate(allocation, server) {
      const taskGroup = server.db.taskGroups.findBy({
        name: allocation.taskGroup,
      });
      const resources = taskGroup.taskIds.map((id) => {
        const task = server.db.tasks.find(id);
        return server.create('task-resource', {
          allocation,
          name: task.name,
          resources: generateResources({
            CPU: task.resources.CPU,
            MemoryMB: task.resources.MemoryMB,
            DiskMB: task.resources.DiskMB,
            networks: { minPorts: 0, maxPorts: 0 },
          }),
        });
      });

      allocation.update({ taskResourceIds: resources.mapBy('id') });
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
        (allocation.rescheduleTracker && allocation.rescheduleTracker.Events) ||
        [];

      let rescheduleTime;
      if (previousEvents.length) {
        const lastEvent = previousEvents[previousEvents.length - 1];
        rescheduleTime = moment(lastEvent.RescheduleTime / 1000000).add(
          5,
          'minutes'
        );
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
          shallow: allocation.shallow,
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
          shallow: allocation.shallow,
          rescheduleTracker,
        });
      }

      allocation.update({
        nextAllocation: nextAllocation.id,
        clientStatus: 'failed',
      });
    },
  }),

  preempted: trait({
    afterCreate(allocation, server) {
      const preempter = server.create('allocation', {
        preemptedAllocations: [allocation.id],
      });
      allocation.update({ preemptedByAllocation: preempter.id });
    },
  }),

  preempter: trait({
    afterCreate(allocation, server) {
      const preempted = server.create('allocation', {
        preemptedByAllocation: allocation.id,
      });
      allocation.update({ preemptedAllocations: [preempted.id] });
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

    const job = allocation.jobId
      ? server.db.jobs.find(allocation.jobId)
      : pickOne(server.db.jobs);
    const namespace = allocation.namespace || job.namespace;
    const node = allocation.nodeId
      ? server.db.nodes.find(allocation.nodeId)
      : pickOne(server.db.nodes);
    const taskGroup = allocation.taskGroup
      ? server.db.taskGroups.findBy({ name: allocation.taskGroup })
      : pickOne(server.db.taskGroups.where({ jobId: job.id }));

    allocation.update({
      namespace,
      jobId: job.id,
      nodeId: node.id,
      taskStateIds: [],
      taskResourceIds: [],
      taskGroup: taskGroup.name,
      name: allocation.name || `${taskGroup.name}.[${faker.random.number(10)}]`,
    });

    if (!allocation.shallow) {
      const states = taskGroup.taskIds.map((id) =>
        server.create('task-state', {
          allocation,
          name: server.db.tasks.find(id).name,
        })
      );

      const resources = taskGroup.taskIds.map((id) => {
        const task = server.db.tasks.find(id);
        return server.create('task-resource', {
          allocation,
          name: task.name,
          resources: task.originalResources,
        });
      });

      allocation.update({
        taskStateIds:
          allocation.clientStatus === 'pending' ? [] : states.mapBy('id'),
        taskResourceIds: resources.mapBy('id'),
      });

      // Each allocation has a corresponding allocation stats running on some client.
      // Create that record, even though it's not a relationship.
      server.create('client-allocation-stat', {
        id: allocation.id,
        _taskNames: states.mapBy('name'),
      });
    }
  },
});
